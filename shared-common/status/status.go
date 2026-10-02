package status

import "time"

const Active = "ATIVO"

// Response representa a resposta de status compartilhada entre os serviços.
type Response struct {
	Service string    `json:"servico"`
	Status  string    `json:"status"`
	Instant time.Time `json:"instante"`
}

// New cria uma resposta de serviço ativo usando horário UTC.
func New(service string) Response {
	return Response{
		Service: service,
		Status:  Active,
		Instant: time.Now().UTC(),
	}
}
