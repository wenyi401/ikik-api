package config

// ReceiptCodeStorageConfig configures the object store used for payout receipt images.
type ReceiptCodeStorageConfig struct {
	Enabled              bool   `mapstructure:"enabled"`
	Endpoint             string `mapstructure:"endpoint"`
	Region               string `mapstructure:"region"`
	Bucket               string `mapstructure:"bucket"`
	AccessKeyID          string `mapstructure:"access_key_id"`
	SecretAccessKey      string `mapstructure:"secret_access_key"`
	Prefix               string `mapstructure:"prefix"`
	PublicBaseURL        string `mapstructure:"public_base_url"`
	ForcePathStyle       bool   `mapstructure:"force_path_style"`
	MaxSizeBytes         int64  `mapstructure:"max_size_bytes"`
	PresignExpireSeconds int    `mapstructure:"presign_expire_seconds"`
}
