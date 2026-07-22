package objectstorage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestPresignedPutAndHead(t *testing.T) {
	if os.Getenv("TEST_OBJECT_STORAGE") == "" {
		t.Skip("TEST_OBJECT_STORAGE is not set")
	}
	storage, err := New(context.Background(), "http://localhost:9000", "http://localhost:9000", "us-east-1", "bjj-videos", "minioadmin", "minioadmin", true, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("small browser-compatible test placeholder")
	key := fmt.Sprintf("integration/%d.mp4", time.Now().UnixNano())
	url, err := storage.PresignPut(context.Background(), key, "video/mp4", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "video/mp4")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("put status=%d", response.StatusCode)
	}
	object, err := storage.Head(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if object.Size != int64(len(body)) || object.ContentType != "video/mp4" {
		t.Fatalf("object=%+v", object)
	}
	getURL, err := storage.PresignGet(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	getRequest, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	getRequest.Header.Set("Range", "bytes=0-4")
	getResponse, err := http.DefaultClient.Do(getRequest)
	if err != nil {
		t.Fatal(err)
	}
	getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusPartialContent || getResponse.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("range status=%d accept-ranges=%q", getResponse.StatusCode, getResponse.Header.Get("Accept-Ranges"))
	}
}
