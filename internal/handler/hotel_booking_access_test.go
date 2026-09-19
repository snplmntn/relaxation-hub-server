package handler

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHotelBookingRoutes(t *testing.T) {
	for _, test := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/api/v1/hotel/bookings", true}, {"GET", "/api/v1/hotel/analytics", true}, {"PATCH", "/api/v1/hotel/bookings/12", true}, {"POST", "/api/v1/hotel/bookings/12/cancel", true},
		{"PATCH", "/api/v1/bookings/12", false}, {"GET", "/api/v1/bookings", false}, {"POST", "/api/v1/hotel/bookings/12/assign", false},
		{"PATCH", "/api/v1/hotel/bookings/nope", false}, {"DELETE", "/api/v1/hotel/bookings/12", false},
	} {
		require.Equal(t, test.allowed, hotelAccountRequestAllowed(test.method, test.path), test.method+" "+test.path)
	}
}
