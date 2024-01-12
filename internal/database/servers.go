package database

type Server struct {
	Id       int          `json:"id"`
	Host     string       `json:"host" validate:"required,max=128"`
	Port     int          `json:"port" validate:"required,min=1,max=65536"`
	Password string       `json:"password" validate:"required"`
	Method   string       `json:"method" validate:"required,in:chacha20-ietf-poly1305"`
	Status   ServerStatus `json:"status"`
}

type ServerStatus string

const (
	ServerStatusProcessing  ServerStatus = "processing"
	ServerStatusAvailable                = "available"
	ServerStatusUnavailable              = "unavailable"
	ServerStatusUnstable                 = "unstable"
)
