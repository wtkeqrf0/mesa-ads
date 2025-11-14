package configs

// HTTP defines configuration for the HTTP server. The Port specifies
// which port the server will bind to. The host may be configured via
// the ADDRESS environment variable, but port is sufficient for most use
// cases.
type HTTP struct {
	// Port is the TCP port the HTTP server will listen on. Defaults to 8080.
	Port uint16 `env:"PORT" envDefault:"8080"`
}
