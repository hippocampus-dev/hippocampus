package routes

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/xerrors"
)

func parseSlackTimestamp(timestampStr string) (*time.Time, error) {
	if timestampStr == "" {
		return nil, nil
	}

	timestampFloat, err := strconv.ParseFloat(timestampStr, 64)
	if err != nil {
		return nil, err
	}

	timestamp := time.Unix(int64(timestampFloat), int64((timestampFloat-float64(int64(timestampFloat)))*1e9))
	return &timestamp, nil
}

func encodeCursor(offset int) string {
	data := fmt.Sprintf("offset:%d", offset)
	return base64.URLEncoding.EncodeToString([]byte(data))
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, xerrors.Errorf("invalid cursor format: %w", err)
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 || parts[0] != "offset" {
		return 0, xerrors.Errorf("invalid cursor content: expected 'offset:N', got '%s'", string(decoded))
	}

	offset, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, xerrors.Errorf("invalid offset value: %w", err)
	}

	return offset, nil
}
