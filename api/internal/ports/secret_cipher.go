package ports

type SecretCipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}
