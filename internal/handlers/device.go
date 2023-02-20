package handlers

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// ListDevices lists all devices
// @Summary      List Devices
// @Description  Lists all devices
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Device
// @Failure		 401  {object}  models.ApiError
// @Router       /devices [get]
func (api *API) ListDevices(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevices")
	defer span.End()
	devices := make([]models.Device, 0)
	result := api.db.WithContext(ctx).Scopes(FilterAndPaginate(&models.Device{}, c)).Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// GetDevice gets a device by ID
// @Summary      Get Devices
// @Description  Gets a device by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Router       /devices/{id} [get]
func (api *API) GetDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "id is not valid"})
		return
	}
	var device models.Device
	result := api.db.WithContext(ctx).First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, device)
}

// UpdateDevice updates a Device
// @Summary      Update Devices
// @Description  Updates a device by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Param		 update body models.UpdateDevice true "Device Update"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Router       /devices/{id} [get]
func (api *API) UpdateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "id is not valid"})
		return
	}
	var request models.UpdateDevice

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}

	var device models.Device
	result := api.db.WithContext(ctx).First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}

	if request.EndpointLocalAddressIPv4 != "" {
		device.EndpointLocalAddressIPv4 = request.EndpointLocalAddressIPv4
	}

	if request.Hostname != "" {
		device.Hostname = request.Hostname
	}

	if request.LocalIP != "" {
		device.LocalIP = request.LocalIP
	}

	if request.OrganizationID != uuid.Nil {
		device.OrganizationID = request.OrganizationID
	}

	if request.ReflexiveIPv4 != "" {
		device.ReflexiveIPv4 = request.ReflexiveIPv4
	}

	if request.SymmetricNat != device.SymmetricNat {
		device.SymmetricNat = request.SymmetricNat
	}

	api.db.Save(&device)

	c.JSON(http.StatusOK, device)
}

// CreateDevice handles adding a new device
// @Summary      Add Devices
// @Description  Adds a new device
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        device  body   models.AddDevice  true "Add Device"
// @Success      201  {object}  models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      409  {object}  models.Device
// @Failure      500  {object}  models.ApiError
// @Router       /devices [post]
func (api *API) CreateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateDevice")
	defer span.End()
	var request models.AddDevice
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}
	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the request did not contain a valid public key"})
		return
	}

	userId := c.GetString(gin.AuthUserKey)
	tx := api.db.Begin().WithContext(ctx)

	var user models.User
	if res := tx.Preload("Devices").Preload("Organizations").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "user not found"})
		return
	}

	var org models.Organization
	result := tx.First(&org, "id = ?", request.OrganizationID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.ApiError{Error: "organization not found"})
		return
	}

	var device models.Device
	res := tx.Where("public_key = ?", request.PublicKey).First(&device)
	if res.Error == nil {
		c.JSON(http.StatusConflict, device)
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}

	var permitted bool
	for _, org := range user.Organizations {
		if org.ID == request.OrganizationID {
			permitted = true
		}
	}
	if !permitted {
		c.JSON(http.StatusForbidden, models.ApiError{Error: fmt.Sprintf("user is not a member of organization: %s", request.OrganizationID.String())})
		return
	}

	ipamPrefix := org.IpCidr
	var relay bool
	// determine if the node joining is a relay or in a hub-zone
	if request.Relay && org.HubZone {
		relay = true
	}

	var ipamIP string
	// If this was a static address request
	// TODO: handle a user requesting an IP not in the IPAM prefix
	if device.TunnelIP != "" {
		var err error
		ipamIP, err = api.ipam.AssignSpecificTunnelIP(ctx, org.ID.String(), ipamPrefix, device.TunnelIP)
		if err != nil {
			tx.Rollback()
			api.Logger(ctx).Error(err)
			c.JSON(http.StatusInternalServerError, models.ApiError{Error: fmt.Sprintf("failed to request specific ipam address: %v", err)})
			return
		}
	} else {
		var err error
		ipamIP, err = api.ipam.AssignFromPool(ctx, org.ID.String(), ipamPrefix)
		if err != nil {
			tx.Rollback()
			api.Logger(ctx).Error(err)
			c.JSON(http.StatusInternalServerError, models.ApiError{Error: fmt.Sprintf("failed to request ipam address: %v", err)})
			return
		}
	}
	// allocate a child prefix if requested
	for _, prefix := range device.ChildPrefix {
		if err := api.ipam.AssignPrefix(ctx, org.ID.String(), prefix); err != nil {
			tx.Rollback()
			api.Logger(ctx).Error(err)
			c.JSON(http.StatusInternalServerError, models.ApiError{Error: fmt.Sprintf("failed to assign child prefix: %v", err)})
			return
		}
	}

	// append a /32 to the IPAM assignment unless it is a relay prefix
	hostPrefix := ipamIP
	if net.ParseIP(ipamIP) != nil && !relay {
		hostPrefix = fmt.Sprintf("%s/32", ipamIP)
	}

	var allowedIPs []string
	allowedIPs = append(allowedIPs, hostPrefix)

	device = models.Device{
		UserID:                   user.ID,
		OrganizationID:           org.ID,
		PublicKey:                request.PublicKey,
		LocalIP:                  request.LocalIP,
		AllowedIPs:               allowedIPs,
		TunnelIP:                 ipamIP,
		ChildPrefix:              request.ChildPrefix,
		Relay:                    request.Relay,
		OrganizationPrefix:       org.IpCidr,
		ReflexiveIPv4:            request.ReflexiveIPv4,
		EndpointLocalAddressIPv4: request.EndpointLocalAddressIPv4,
		SymmetricNat:             request.SymmetricNat,
		Hostname:                 request.Hostname,
	}

	if res := tx.Create(&device); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: res.Error.Error()})
		return
	}
	if err := tx.Commit(); err.Error != nil {
		tx.Rollback()
		api.Logger(ctx).Error(err.Error)
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}
	span.SetAttributes(
		attribute.String("id", device.ID.String()),
	)
	c.JSON(http.StatusCreated, device)
}

// DeleteDevice handles deleting an existing device and associated ipam lease
// @Summary      Delete Device
// @Description  Deletes an existing device and associated IPAM lease
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      204  {object}  models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 400  {object}  models.ApiError
// @Failure		 400  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /devices/{id} [delete]
func (api *API) DeleteDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteDevice")
	defer span.End()
	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "device id is not valid"})
		return
	}

	baseID := models.Base{ID: deviceID}
	device := models.Device{}
	device.Base = baseID
	ipamAddress := device.TunnelIP
	orgID := device.OrganizationID
	orgPrefix := device.OrganizationPrefix
	childPrefix := device.ChildPrefix

	if res := api.db.WithContext(ctx).Delete(&device, "id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}

	if ipamAddress != "" && orgPrefix != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), orgID.String(), ipamAddress, orgPrefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.ApiError{
				Error: fmt.Sprintf("%v", err),
			})
		}
	}

	for _, prefix := range childPrefix {
		if err := api.ipam.ReleasePrefix(c.Request.Context(), orgID.String(), prefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.ApiError{
				Error: fmt.Sprintf("failed to release child prefix: %v", err),
			})
		}
	}

	c.JSON(http.StatusOK, device)
}
