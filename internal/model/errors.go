package model

import "errors"

var ErrNotFound = errors.New("not found")

var ErrInvalidStaffOutTimeTargetRole = errors.New("staff out time target role is not eligible")

var ErrStaffOutTimeOutsideWorkDateWindow = errors.New("staff out time is outside the work date window")

var ErrInvalidStaffAttendanceTargetRole = errors.New("staff attendance target role is not eligible")
var ErrStaffAttendanceOutsideWorkDateWindow = errors.New("staff attendance is outside the work date window")
var ErrStaffAttendanceTimeOutBeforeTimeIn = errors.New("staff attendance time-out must be after time-in")
var ErrStaffAttendanceShiftTooLong = errors.New("staff attendance shift exceeds maximum duration")
var ErrStaffAttendanceLocked = errors.New("staff attendance is locked by approved or paid payroll")
var ErrStaffAttendanceSelfEditForbidden = errors.New("admins cannot manage their own attendance")
var ErrStaffAttendanceDuplicate = errors.New("active staff attendance already exists for user and work date")
var ErrInvalidPayrollRole = errors.New("payroll role is not eligible")
var ErrPayrollRunHasBlockers = errors.New("payroll run has blocking rows")
var ErrPayrollRunImmutable = errors.New("payroll run cannot be modified")
var ErrPayrollPaymentMethodRequired = errors.New("payroll payment method is required")
var ErrInvalidPayrollPaymentMethod = errors.New("payroll payment method is invalid")
var ErrPayrollRateLocked = errors.New("payroll rate is locked by approved or paid payroll")
var ErrPayrollAdjustmentLocked = errors.New("payroll adjustment is locked by approved or paid payroll")
var ErrInvalidPayrollRate = errors.New("payroll rate payload is invalid")
var ErrInvalidPayrollAdjustment = errors.New("payroll adjustment payload is invalid")
