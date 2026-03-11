package postgres

import "fmt"

type Config struct {
	Host     string `mapstructure:"host" default:"localhost" validate:"required"`
	Port     int    `mapstructure:"port" default:"5432" validate:"required,min=1,max=65535"`
	Username string `mapstructure:"username" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.Username, c.Password, c.Host, c.Port, c.Database)
}
