package clients

import "net/http"
import "os"

func GetCredential(request *http.Request) string {
	credential := request.Header.Get("X-Vtex-Credential")
	if credential != "" {
		return credential
	}

	return os.Getenv("VTEX_CREDENTIAL")
}
