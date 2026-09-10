package service

import "github.com/snplmntn/relaxation-hub-server/internal/repository"

func (s *BookingService) SetHotelBookingRepository(repo repository.HotelBookingRepository) {
	s.hotelBookingRepo = repo
}
