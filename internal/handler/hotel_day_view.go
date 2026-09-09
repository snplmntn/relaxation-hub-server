package handler

import (
	"errors"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"net/http"
)

type HotelDayViewHandler struct{ service *service.HotelDayViewService }

func NewHotelDayViewHandler(s *service.HotelDayViewService) *HotelDayViewHandler {
	return &HotelDayViewHandler{service: s}
}

func (h *HotelDayViewHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	view, err := h.service.Get(r.Context(), id, r.URL.Query().Get("date"))
	if errors.Is(err, service.ErrHotelDayViewDate) {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		respondHotelAccessError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, view)
}
