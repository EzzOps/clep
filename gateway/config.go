package main

import "os"

func loadConfig() Config {
	return Config{
		Port:           getEnvInt("PORT", 8081),
		NatsURL:        getEnv("NATS_URL", "nats://nats:4222"),
		PlaneSecret:    getEnv("PLANE_WEBHOOK_SECRET", ""),
		PlaneAPIURL:    getEnv("PLANE_API_URL", ""),
		PlaneAPIKey:    getEnv("PLANE_API_KEY", ""),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		HermesAdapter:  getEnv("HERMES_ADAPTER_URL", "http://hermes:8080"),
		EventStream:    getEnv("EVENT_STREAM", "clep.events"),
		ResponseStream: getEnv("RESPONSE_STREAM", "clep.responses"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		var n int
		fmt.Sscanf(val, "%d", &n)
		if n != 0 {
			return n
		}
	}
	return defaultVal
}
