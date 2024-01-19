package database

type ServerStatus string

const (
	ServerStatusProcessing  ServerStatus = "processing"
	ServerStatusAvailable                = "available"
	ServerStatusUnavailable              = "unavailable"
)

type Server struct {
	Id           int          `json:"id"`
	Host         string       `json:"host" validate:"required,max=128"`
	HttpToken    string       `json:"http_token" validate:"required"`
	HttpPort     int          `json:"http_port" validate:"required,min=1,max=65536"`
	SsRemotePort int          `json:"ss_remote_port" validate:"required,min=1,max=65536"`
	SsLocalPort  int          `json:"ss_local_port" validate:"required,min=0,max=65536"`
	Status       ServerStatus `json:"status"`
	Traffic      int64        `json:"traffic"`
}
