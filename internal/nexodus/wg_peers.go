package nexodus

import (
	"net"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	// How long to wait for successful peering after choosing a new peering method
	peeringTimeout = time.Second * 30
	// How long to wait for peering to successfully restore itself after seeing
	// successful peering using a given method, but it goes down.
	peeringRestoreTimeout             = time.Second * 180
	peeringMethodRelaySelfDirectLocal = "relay-node-self-direct-local"
	peeringMethodRelaySelf            = "relay-node-self"
	peeringMethodRelayPeerDirectLocal = "relay-node-peer-direct-local"
	peeringMethodRelayPeer            = "relay-node-peer"
	peeringMethodDirectLocal          = "direct-local"
	peeringMethodReflexive            = "reflexive"
	peeringMethodViaRelay             = "via-relay"
	peeringMethodNone                 = "none"
)

type wgPeerMethod struct {
	// name of the peering method
	name string
	// determine if this peering method is available for the given device
	checkPrereqs func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string, healthyRelay bool) bool
	// buildPeerConfig builds the peer configuration for a given peering method
	buildPeerConfig func(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, peerPort, reflexiveIP4 string) wgPeerConfig
}

// list of peering methods, in order of preference
var wgPeerMethods = []wgPeerMethod{
	{
		// This node is a relay node and we have the same reflexive address as the peer
		name: peeringMethodRelaySelfDirectLocal,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string, healthyRelay bool) bool {
			return nx.relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeerForRelayNode,
	},
	{
		// This node is a relay node
		name: peeringMethodRelaySelf,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string, healthyRelay bool) bool {
			return nx.relay
		},
		buildPeerConfig: buildPeerForRelayNode,
	},
	{
		// The peer is a relay node and we have the same reflexive address as the peer
		name: peeringMethodRelayPeerDirectLocal,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string, healthyRelay bool) bool {
			return !nx.relay && device.Relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalRelayPeer,
	},
	{
		// The peer is a relay node
		name: peeringMethodRelayPeer,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string, healthyRelay bool) bool {
			return !nx.relay && device.Relay
		},
		buildPeerConfig: buildRelayPeer,
	},
	{
		// We are behind the same reflexive address as the peer, try direct, local peering
		name: peeringMethodDirectLocal,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, reflexiveIP4 string, healthyRelay bool) bool {
			return !nx.relay && !device.Relay && nx.nodeReflexiveAddressIPv4.Addr().String() == parseIPfromAddrPort(reflexiveIP4)
		},
		buildPeerConfig: buildDirectLocalPeer,
	},
	{
		// If neither side is behind symmetric NAT, we can try peering with its reflexive address.
		// This is the address+port opened up by the peer using STUN.
		name: peeringMethodReflexive,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string, healthyRelay bool) bool {
			return !nx.relay && !device.Relay && !device.SymmetricNat && !nx.symmetricNat
		},
		buildPeerConfig: buildReflexivePeer,
	},
	{
		// Last chance, try connecting to the peer via a relay
		name: peeringMethodViaRelay,
		checkPrereqs: func(nx *Nexodus, device public.ModelsDevice, _ string, healthyRelay bool) bool {
			return !nx.relay && !device.Relay && healthyRelay
		},
		buildPeerConfig: func(nx *Nexodus, device public.ModelsDevice, _ []string, _, _, _ string) wgPeerConfig {
			return wgPeerConfig{
				AllowedIPsForRelay: device.AdvertiseCidrs,
			}
		},
	},
}

func (nx *Nexodus) peeringReset(d *deviceCacheEntry) {
	nx.logger.Debugf("Resetting peer configuration - Peer AllowedIps [ %s ] Peer Public Key [ %s ]",
		strings.Join(d.device.AllowedIps, ", "), d.device.PublicKey)

	d.peeringMethod = peeringMethodNone
	// By setting the peering method index to -1, we will consider all other
	// methods that may be available.
	d.peeringMethodIndex = -1

	// All the stats are now invalid
	d.peeringTime = time.Time{}
	d.peerHealthy = false
	d.peerHealthyTime = time.Time{}
	d.lastTxBytes = 0
	d.lastRxBytes = 0
	d.lastHandshakeTime = time.Time{}
	d.lastHandshake = ""
	d.lastRefresh = time.Time{}
}

// shouldResetPeering() determines if we should reset peering to start over at the
// beginning of the peering list.
func (nx *Nexodus) shouldResetPeering(d *deviceCacheEntry, reflexiveIP4 string, healthyRelay bool) bool {
	if d.peeringMethodIndex == -1 {
		// Already in a reset state
		return false
	}

	if d.peeringMethodIndex == len(wgPeerMethods)-1 {
		// We've reached the end of the peering method list, time to reset
		return true
	}

	// If not at the end, check to see if the prerequisites pass for any of the following methods
	for i := d.peeringMethodIndex + 1; i < len(wgPeerMethods); i++ {
		if wgPeerMethods[i].checkPrereqs(nx, d.device, reflexiveIP4, healthyRelay) {
			// Prequisites pass for this method, so don't reset
			return false
		}
	}

	// There are no methods remaining that have passed the prerequisites, so reset
	return true
}

func (nx *Nexodus) rebuildPeerConfig(d *deviceCacheEntry, healthyRelay bool) (wgPeerConfig, string, int) {
	localIP, reflexiveIP4 := nx.extractLocalAndReflexiveIP(d.device)
	peerPort := nx.extractPeerPort(localIP)
	relayAllowedIP := []string{
		nx.vpc.Ipv4Cidr,
		nx.vpc.Ipv6Cidr,
	}

	tryNextMethod := nx.peeringFailed(*d, healthyRelay)
	if tryNextMethod {
		nx.logger.Debugf("Peering with peer [ %s ] using method [ %s ] has failed, trying next method", d.device.PublicKey, d.peeringMethod)
		if nx.shouldResetPeering(d, reflexiveIP4, healthyRelay) {
			// We failed to connect via a relay, which is the last resort, so start over at the beginning
			nx.peeringReset(d)
			tryNextMethod = false
		}
	}

	peer := nx.wgConfig.Peers[d.device.PublicKey]
	chosenMethod := d.peeringMethod
	chosenMethodIndex := d.peeringMethodIndex
	for i, method := range wgPeerMethods {
		if i < d.peeringMethodIndex {
			// A peering method was previously chosen and we haven't reached it yet
			continue
		}
		if i == d.peeringMethodIndex && tryNextMethod {
			// A peering method was previously chosen and it failed
			continue
		}
		if !method.checkPrereqs(nx, d.device, reflexiveIP4, healthyRelay) {
			// This peering method is not a candidate for this peer, the prereqs failed
			continue
		}
		if method.name == peeringMethodViaRelay && d.peeringMethod == peeringMethodViaRelay {
			// We are already set up to use a relay for this peer
			break
		}
		peer = method.buildPeerConfig(nx, d.device, relayAllowedIP, localIP, peerPort, reflexiveIP4)
		chosenMethod = method.name
		chosenMethodIndex = i
		break
	}

	return peer, chosenMethod, chosenMethodIndex
}

// buildPeersConfig builds the peer configuration based off peer cache
// and peer listings from the controller. assumes deviceCacheLock is held.
// Returns a map of peer public keys to devices that have had their wireguard
// configuration updated.
func (nx *Nexodus) buildPeersConfig() map[string]public.ModelsDevice {
	if nx.wgConfig.Peers == nil {
		nx.wgConfig.Peers = map[string]wgPeerConfig{}
	}

	updatedPeers := map[string]public.ModelsDevice{}
	allowedIPsForRelay := []string{}

	_, nx.wireguardPubKeyInConfig = nx.deviceCache[nx.wireguardPubKey]

	nx.buildLocalConfig()

	// do we have a healthy relay available?
	healthyRelay := false
	relayDevice := public.ModelsDevice{}
	for _, d := range nx.deviceCache {
		if d.device.Relay && d.peerHealthy {
			healthyRelay = true
			relayDevice = d.device
			break
		}
	}

	now := time.Now()
	for _, dIter := range nx.deviceCache {
		d := dIter
		// skip ourselves
		if d.device.PublicKey == nx.wireguardPubKey {
			continue
		}

		peerConfig, chosenMethod, chosenMethodIndex := nx.rebuildPeerConfig(&d, healthyRelay)
		if len(peerConfig.AllowedIPsForRelay) > 0 {
			allowedIPsForRelay = append(allowedIPsForRelay, peerConfig.AllowedIPsForRelay...)
		}

		if !nx.peerConfigUpdated(d.device, peerConfig) {
			// The resulting peer configuration hasn't changed.
			continue
		}

		updatedPeers[d.device.PublicKey] = d.device
		if chosenMethod == peeringMethodViaRelay {
			// When switching to a relay, we have no configuration to connect directly to the peer
			if _, ok := nx.wgConfig.Peers[d.device.PublicKey]; ok {
				delete(nx.wgConfig.Peers, d.device.PublicKey)
				_ = nx.peerCleanup(d.device)
			}
		} else {
			nx.wgConfig.Peers[d.device.PublicKey] = peerConfig
		}
		d.peeringMethodIndex = chosenMethodIndex
		d.peeringMethod = chosenMethod
		d.peeringTime = now
		nx.deviceCache[d.device.PublicKey] = d
		nx.logPeerInfo(d.device, peerConfig.Endpoint, chosenMethod)
	}

	if healthyRelay && len(allowedIPsForRelay) > 0 {
		// Add child prefix CIDRs to the relay for peers that we can only reach via the relay
		relayConfig := nx.wgConfig.Peers[relayDevice.PublicKey]
		relayConfig.AllowedIPs = append([]string{nx.vpc.Ipv4Cidr, nx.vpc.Ipv4Cidr}, allowedIPsForRelay...)
		nx.wgConfig.Peers[relayDevice.PublicKey] = relayConfig
	}

	return updatedPeers
}

func (nx *Nexodus) peeringFailed(d deviceCacheEntry, healthyRelay bool) bool {
	if d.peerHealthy {
		return false
	}

	if d.peeringMethodIndex == len(wgPeerMethods)-1 {
		return !healthyRelay
	}

	if d.peeringTime.IsZero() {
		// haven't even tried yet ...
		return false
	}

	if d.peerHealthyTime.IsZero() && time.Since(d.peeringTime) < peeringTimeout {
		// Peering has never been successful since choosing this method,
		// so time out quicker than if it had worked and we're waiting for it to come back up.
		return false
	}

	if !d.peerHealthyTime.IsZero() && time.Since(d.peerHealthyTime) < peeringRestoreTimeout {
		// Peering worked, but went down, so give it a few minutes to come back up.
		return false
	}

	return true
}

func (nx *Nexodus) peerConfigUpdated(device public.ModelsDevice, peer wgPeerConfig) bool {
	if _, ok := nx.wgConfig.Peers[device.PublicKey]; !ok {
		return true
	}

	if nx.wgConfig.Peers[device.PublicKey].Endpoint != peer.Endpoint {
		return true
	}

	if !reflect.DeepEqual(nx.wgConfig.Peers[device.PublicKey].AllowedIPs, peer.AllowedIPs) {
		return true
	}

	return false
}

// extractLocalAndReflexiveIP retrieve the local and reflexive endpoint addresses
func (nx *Nexodus) extractLocalAndReflexiveIP(device public.ModelsDevice) (string, string) {
	localIP := ""
	reflexiveIP4 := ""
	for _, endpoint := range device.Endpoints {
		if endpoint.Source == "local" {
			localIP = endpoint.Address
		} else {
			reflexiveIP4 = endpoint.Address
		}
	}
	return localIP, reflexiveIP4
}

func (nx *Nexodus) extractPeerPort(localIP string) string {
	_, port, err := net.SplitHostPort(localIP)
	if err != nil {
		nx.logger.Debugf("failed parse the endpoint address for node (likely still converging) : %v", err)
		return ""
	}
	return port
}

func buildDirectLocalPeerForRelayNode(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            localIP,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildPeerForRelayNode build a config for all peers if this node is the organization's relay node.
// The peer for a relay node is currently left blank and assumed to be exposed to all peers, we still build its peer config for flexibility.
func buildPeerForRelayNode(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func buildDirectLocalRelayPeer(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            localIP,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildRelayPeer Build the relay peer entry that will be a CIDR block as opposed to a /32 host route. All nodes get this peer.
// This is the only peer a symmetric NAT node will get unless it also has a direct peering
func buildRelayPeer(nx *Nexodus, device public.ModelsDevice, relayAllowedIP []string, localIP, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          relayAllowedIP,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildDirectLocalPeer If both nodes are local, peer them directly to one another via their local addresses (includes symmetric nat nodes)
// The exception is if the peer is a relay node since that will get a peering with the org prefix supernet
func buildDirectLocalPeer(nx *Nexodus, device public.ModelsDevice, _ []string, localIP, _, _ string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            localIP,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

// buildReflexive Peer the bulk of the peers will be added here except for local address peers or
// symmetric NAT peers or if this device is itself a symmetric nat node, that require relaying.
func buildReflexivePeer(nx *Nexodus, device public.ModelsDevice, _ []string, _, _, reflexiveIP4 string) wgPeerConfig {
	device.AllowedIps = append(device.AllowedIps, device.AdvertiseCidrs...)
	return wgPeerConfig{
		PublicKey:           device.PublicKey,
		Endpoint:            reflexiveIP4,
		AllowedIPs:          device.AllowedIps,
		PersistentKeepAlive: persistentKeepalive,
	}
}

func (nx *Nexodus) logPeerInfo(device public.ModelsDevice, endpointIP, method string) {
	nx.logger.Debugf("Peer configuration - Method [ %s ] Peer AllowedIps [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]",
		method,
		strings.Join(device.AllowedIps, ", "),
		endpointIP,
		device.PublicKey)
}

// buildLocalConfig builds the configuration for the local interface
func (nx *Nexodus) buildLocalConfig() {
	var localInterface wgLocalConfig
	var d deviceCacheEntry
	var ok bool

	if d, ok = nx.deviceCache[nx.wireguardPubKey]; !ok {
		return
	}

	// if the local node address changed replace it on wg0
	if nx.TunnelIP != d.device.Ipv4TunnelIps[0].Address {
		nx.logger.Infof("New local Wireguard interface addresses assigned IPv4 [ %s ] IPv6 [ %s ]", d.device.Ipv4TunnelIps[0].Address, d.device.Ipv6TunnelIps[0].Address)
		if runtime.GOOS == Linux.String() && linkExists(nx.tunnelIface) {
			if err := delLink(nx.tunnelIface); err != nil {
				nx.logger.Infof("Failed to delete %s: %v", nx.tunnelIface, err)
			}
		}
	}
	nx.TunnelIP = d.device.Ipv4TunnelIps[0].Address
	nx.TunnelIpV6 = d.device.Ipv6TunnelIps[0].Address
	localInterface = wgLocalConfig{
		nx.wireguardPvtKey,
		nx.listenPort,
	}
	// set the node unique local interface configuration
	nx.wgConfig.Interface = localInterface
}
