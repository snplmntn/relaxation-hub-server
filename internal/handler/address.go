package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AddressHandler struct {
	addressService *service.AddressService
}

func NewAddressHandler(addressService *service.AddressService) *AddressHandler {
	return &AddressHandler{
		addressService: addressService,
	}
}

// AdminListUserAddresses allows admins to list addresses for a specific user.
func (h *AddressHandler) AdminListUserAddresses(w http.ResponseWriter, r *http.Request) {
	// target userID from path
	targetUserID, err := parseIDFromPath(r, "userId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	addresses, err := h.addressService.List(r.Context(), targetUserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]model.AddressResponse, 0, len(addresses))
	for i := range addresses {
		responses = append(responses, toAddressResponse(&addresses[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// AdminCreateUserAddress allows admins to create an address for a specific user.
func (h *AddressHandler) AdminCreateUserAddress(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := parseIDFromPath(r, "userId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req model.CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	addr, err := h.addressService.Create(r.Context(), targetUserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toAddressResponse(addr))
}

func (h *AddressHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	addr, err := h.addressService.Create(r.Context(), userID, &req)
	if err != nil {
		// Log the actual error for debugging
		println("CreateAddress error:", err.Error())
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toAddressResponse(addr))
}

func (h *AddressHandler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	addresses, err := h.addressService.List(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]model.AddressResponse, 0, len(addresses))
	for i := range addresses {
		responses = append(responses, toAddressResponse(&addresses[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (h *AddressHandler) GetAddress(w http.ResponseWriter, r *http.Request) {
	addressID, err := parseAddressID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid address id")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	addr, err := h.addressService.GetByID(r.Context(), addressID, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "address not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toAddressResponse(addr))
}

func (h *AddressHandler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	addressID, err := parseAddressID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid address id")
		return
	}

	var req model.UpdateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	addr, err := h.addressService.Update(r.Context(), addressID, userID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "address not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toAddressResponse(addr))
}

func (h *AddressHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	addressID, err := parseAddressID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid address id")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.addressService.Delete(r.Context(), addressID, userID); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "address not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AddressHandler) SetDefaultAddress(w http.ResponseWriter, r *http.Request) {
	addressID, err := parseAddressID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid address id")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.addressService.SetDefault(r.Context(), addressID, userID); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "address not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "default address updated"})
}

func parseAddressID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	return strconv.ParseInt(idParam, 10, 64)
}

// parseIDFromPath is a small helper for handlers needing path params by key.
func parseIDFromPath(r *http.Request, key string) (int64, error) {
	idParam := chi.URLParam(r, key)
	return strconv.ParseInt(idParam, 10, 64)
}

func toAddressResponse(addr *model.Address) model.AddressResponse {
	return model.AddressResponse{
		AddressID:  addr.AddressID,
		Label:      addr.Label,
		Street:     addr.Street,
		Barangay:   addr.Barangay,
		City:       addr.City,
		Province:   addr.Province,
		PostalCode: addr.PostalCode,
		Landmark:   addr.Landmark,
		Country:    addr.Country,
		Latitude:   addr.Latitude,
		Longitude:  addr.Longitude,
		IsDefault:  addr.IsDefault,
		CreatedAt:  addr.CreatedAt,
		UpdatedAt:  addr.UpdatedAt,
	}
}
