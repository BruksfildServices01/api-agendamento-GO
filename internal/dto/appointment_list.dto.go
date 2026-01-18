package dto

import "time"

type AppointmentListDTO struct {
	ID          uint      `json:"id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Status      string    `json:"status"`
	ClientName  string    `json:"client_name"`
	ProductName string    `json:"product_name"`
}
