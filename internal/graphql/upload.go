package graphql

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Khan/genqlient/graphql"
)

// Upload sends size bytes of body to an upload made with createUpload, and
// returns the upload's id, which a mutation takes in place of the content.
// sha256, the content's digest in lowercase hex, is optional. The URL carries
// its own authorization, so plain must send no credentials.
func Upload(ctx context.Context, c graphql.Client, plain *http.Client, body io.Reader, size int64, sha256 *string) (string, error) {
	resp, err := CreateUpload(ctx, c, int(size), sha256)
	if err != nil {
		return "", err
	}
	target := resp.CreateUpload
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.Url, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = size
	for _, h := range target.Headers {
		req.Header.Add(h.Name, h.Value)
	}
	put, err := plain.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}
	defer put.Body.Close()
	if put.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(put.Body, 4<<10))
		return "", fmt.Errorf("upload: HTTP %d: %s", put.StatusCode, msg)
	}
	return target.Id, nil
}

// Download reads the content at a download URL. The URL carries its own
// authorization, so plain must send no credentials.
func Download(ctx context.Context, plain *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := plain.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
