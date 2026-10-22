package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

func QueryInt(r *http.Request, key string, fallback, minimum, maximum int) (int, error) {
	rawValue := strings.TrimSpace(r.URL.Query().Get(key))
	if rawValue == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil || value < minimum || value > maximum {
		return 0, apperror.Validation(map[string]string{
			key: fmt.Sprintf("must be an integer between %d and %d", minimum, maximum),
		})
	}

	return value, nil
}
