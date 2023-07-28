package nexodus

import (
	"encoding/json"
	"fmt"
)

func (ac *NexdCtl) EnableExitNodeClient(_ string, result *string) error {
	err := ac.ax.ExitNodeClientSetup()

	enableExitNodeClientJson, err := json.Marshal(err)
	if err != nil {
		return fmt.Errorf("error marshalling enable exit node results %w", err)
	}

	*result = string(enableExitNodeClientJson)

	return nil
}

func (ac *NexdCtl) DisableExitNodeClient(_ string, result *string) error {
	err := ac.ax.ExitNodeOriginTeardown()

	disableExitNodeClientJson, err := json.Marshal(err)
	if err != nil {
		return fmt.Errorf("error marshalling disable exit node results %w", err)
	}

	*result = string(disableExitNodeClientJson)

	return nil
}

func (ac *NexdCtl) ListExitNodes(_ string, result *string) error {
	exitNodeOrigins, err := json.Marshal(ac.ax.exitNode.exitNodeOrigins)
	if err != nil {
		return fmt.Errorf("error marshalling disable exit node results %w", err)
	}

	*result = string(exitNodeOrigins)

	return nil
}
