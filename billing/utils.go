package billing

import "os"

func GetGoogleCreds(path string) ([]byte, error) {
	return os.ReadFile(path)
}
