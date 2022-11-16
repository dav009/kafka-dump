package kafka_utils

type Config struct {
	BootstrapServers          string `json:"bootstrap_servers" mapstructure:"bootstrap_servers"`
	SecurityProtocol          string `json:"security_protocol" mapstructure:"security_protocol"`
	SASLMechanism             string `json:"sasl_mechanism" mapstructure:"sasl_mechanism"`
	SASLUsername              string `json:"sasl_username" mapstructure:"sasl_username"`
	SASLPassword              string `json:"sasl_password" mapstructure:"sasl_password"`
	ReadTimeoutSeconds        int16  `json:"read_timeout_seconds" mapstructure:"read_timeout_seconds"`
	GroupId                   string `json:"group_id" mapstructure:"group_id"`
	QueueBufferingMaxMessages int    `json:"queue_buffering_max_messages" mapstructure:"queue_buffering_max_messages"`
}
