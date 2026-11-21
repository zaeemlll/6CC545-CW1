package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/storage"
)

func newStoredFileTestHandler(t *testing.T) (*storedFileHandler, *events.Subscription) {
	t.Helper()

	store, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	bus, err := events.New(8)
	if err != nil {
		t.Fatalf("create event bus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	sub, err := bus.Subscribe("file.added")
	if err != nil {
		t.Fatalf("subscribe to file.added: %v", err)
	}
	t.Cleanup(sub.Cancel)

	return &storedFileHandler{
		store: store, bus: bus, nodeID: "node-a", maxBytes: 1024,
	}, sub
}

func TestStoredFileUploadDownloadAndEvent(t *testing.T) {
	h, sub := newStoredFileTestHandler(t)

	content := []byte("Week 7 stored file")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	upload := httptest.NewRequest(http.MethodPost, "/v1/files/report.txt", bytes.NewReader(content))
	upload.Header.Set("X-File-Size", "18")
	upload.Header.Set("X-File-SHA256", digest)

	uploadResponse := httptest.NewRecorder()
	StoredFile(h.store, h.bus, h.nodeID, h.maxBytes).ServeHTTP(uploadResponse, upload)

	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body: %s",
			uploadResponse.Code, http.StatusCreated, uploadResponse.Body.String())
	}

	var response struct {
		File struct {
			Name   string `json:"Name"`
			Size   int64  `json:"Size"`
			SHA256 string `json:"SHA256"`
		} `json:"file"`
		EventDelivery events.Delivery `json:"event_delivery"`
	}
	if err := json.Unmarshal(uploadResponse.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if response.File.Name != "report.txt" || response.File.Size != int64(len(content)) ||
		response.File.SHA256 != digest {
		t.Fatalf("unexpected upload metadata: %+v", response.File)
	}

	select {
	case event := <-sub.C:
		if event.Kind != "file.added" || event.Name != "report.txt" || event.Origin != "node-a" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("file.added event was not published")
	}

	download := httptest.NewRequest(http.MethodGet, "/v1/files/report.txt", nil)
	downloadResponse := httptest.NewRecorder()
	StoredFile(h.store, h.bus, h.nodeID, h.maxBytes).ServeHTTP(downloadResponse, download)

	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status = %d, want %d; body: %s",
			downloadResponse.Code, http.StatusOK, downloadResponse.Body.String())
	}
	if !bytes.Equal(downloadResponse.Body.Bytes(), content) {
		t.Fatalf("downloaded content = %q, want %q", downloadResponse.Body.Bytes(), content)
	}
	if downloadResponse.Header().Get("X-File-SHA256") != digest {
		t.Fatal("download response has the wrong SHA256 header")
	}
}

func TestStoredFileRejectsInvalidDigestWithoutEvent(t *testing.T) {
	h, sub := newStoredFileTestHandler(t)

	content := []byte("not the claimed content")
	sum := sha256.Sum256([]byte("different content"))

	request := httptest.NewRequest(http.MethodPost, "/v1/files/bad.txt", bytes.NewReader(content))
	request.Header.Set("X-File-Size", "23")
	request.Header.Set("X-File-SHA256", hex.EncodeToString(sum[:]))

	response := httptest.NewRecorder()
	StoredFile(h.store, h.bus, h.nodeID, h.maxBytes).ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	if _, _, err := h.store.Open("bad.txt"); err == nil {
		t.Fatal("invalid file was stored")
	}

	select {
	case event := <-sub.C:
		t.Fatalf("unexpected event after failed upload: %+v", event)
	default:
	}
}

func TestStoredFilesListsCommittedMetadata(t *testing.T) {
	h, _ := newStoredFileTestHandler(t)

	content := []byte("listed file")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	if _, err := h.store.Put(
		context.Background(), "listed.txt", bytes.NewReader(content),
		int64(len(content)), digest,
	); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	response := httptest.NewRecorder()
	StoredFiles(h.store).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body: %s",
			response.Code, http.StatusOK, response.Body.String())
	}

	var files []storage.Metadata
	if err := json.Unmarshal(response.Body.Bytes(), &files); err != nil {
		t.Fatalf("decode file list: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("listed %d files, want 1", len(files))
	}
	if files[0].Name != "listed.txt" ||
		files[0].Size != int64(len(content)) ||
		files[0].SHA256 != digest {
		t.Fatalf("unexpected listed metadata: %+v", files[0])
	}
}

func TestStoredFilesRejectsNonGetMethod(t *testing.T) {
	h, _ := newStoredFileTestHandler(t)

	request := httptest.NewRequest(http.MethodPost, "/v1/files", nil)
	response := httptest.NewRecorder()
	StoredFiles(h.store).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d",
			response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow header = %q, want GET", got)
	}
}

func TestStoredFileRejectsInvalidUploadRequests(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		size       string
		content    []byte
		contentLen int64
		wantStatus int
	}{
		{
			name:       "missing size",
			filename:   "missing.txt",
			content:    []byte("test"),
			contentLen: 4,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid size",
			filename:   "invalid.txt",
			size:       "abc",
			content:    []byte("test"),
			contentLen: 4,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative size",
			filename:   "negative.txt",
			size:       "-1",
			contentLen: 0,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "file too large",
			filename:   "large.txt",
			size:       "1025",
			contentLen: 1025,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "content length mismatch",
			filename:   "mismatch.txt",
			size:       "5",
			content:    []byte("test"),
			contentLen: 4,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid filename",
			filename:   "../unsafe.txt",
			size:       "4",
			content:    []byte("test"),
			contentLen: 4,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported method",
			filename:   "method.txt",
			size:       "4",
			content:    []byte("test"),
			contentLen: 4,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newStoredFileTestHandler(t)

			method := http.MethodPost
			if tc.name == "unsupported method" {
				method = http.MethodPut
			}

			request := httptest.NewRequest(
				method,
				"/v1/files/"+tc.filename,
				bytes.NewReader(tc.content),
			)
			if tc.size != "" {
				request.Header.Set("X-File-Size", tc.size)
			}
			request.Header.Set("X-File-SHA256", hex.EncodeToString(
				sha256.New().Sum(nil),
			))

			if tc.contentLen >= 0 {
				request.ContentLength = tc.contentLen
			}

			response := httptest.NewRecorder()
			StoredFile(h.store, h.bus, h.nodeID, h.maxBytes).
				ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s",
					response.Code, tc.wantStatus, response.Body.String())
			}
		})
	}
}
