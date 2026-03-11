package diskio

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"hash/crc32"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/pkg/cloudauth"
)

// testAPIHandler wraps a mutable handler that also serves auth endpoints.
type testAPIHandler struct {
	handler http.HandlerFunc
}

func (h *testAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/auth/service-account/") {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/begin") {
			json.NewEncoder(w).Encode(cloudauth.BeginAuthResponse{
				Envelope:  "test-envelope",
				Challenge: "test-challenge",
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			json.NewEncoder(w).Encode(cloudauth.CompleteAuthResponse{
				Token:     "test-jwt-token",
				ExpiresIn: 3600,
			})
			return
		}
	}
	if h.handler != nil {
		h.handler(w, r)
	}
}

func newTestUploaderServer(t *testing.T) (*httptest.Server, *testAPIHandler, *cloudauth.AuthClient) {
	t.Helper()
	h := &testAPIHandler{}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	kp, err := cloudauth.GenerateKeyPair()
	require.NoError(t, err)
	authClient, err := cloudauth.NewAuthClient(ts.URL, kp)
	require.NoError(t, err)

	return ts, h, authClient
}

func TestCloudSegmentUploaderFullFlow(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	var (
		uploadReqReceived   bool
		uploadDataReceived  []byte
		completeReqReceived bool
		completeBody        logSegmentCompleteRequest
	)

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			uploadReqReceived = true
			var req logSegmentUploadRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "vol-123", req.VolumeID)
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "cloud-seg-abc",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: "/api/v1/disk/log-segments/cloud-seg-abc/complete",
			})

		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			uploadDataReceived, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)

		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/complete"):
			completeReqReceived = true
			json.NewDecoder(r.Body).Decode(&completeBody)
			w.WriteHeader(http.StatusOK)
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "disk.0001.log")
	segContent := []byte("test segment data for upload")
	require.NoError(t, os.WriteFile(segPath, segContent, 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	segID, err := uploader.UploadSegment(context.Background(), "vol-123", segPath)
	require.NoError(t, err)
	assert.Equal(t, "cloud-seg-abc", segID)

	assert.True(t, uploadReqReceived, "upload request should be sent")
	assert.Equal(t, segContent, uploadDataReceived, "file data should be uploaded")
	assert.True(t, completeReqReceived, "complete request should be sent")

	// Verify hashes
	md5h := md5.Sum(segContent)
	expectedMD5 := base64.StdEncoding.EncodeToString(md5h[:])
	assert.Equal(t, expectedMD5, completeBody.MD5)

	crch := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	crch.Write(segContent)
	expectedCRC := base64.StdEncoding.EncodeToString(crch.Sum(nil))
	assert.Equal(t, expectedCRC, completeBody.CRC32C)

	assert.Equal(t, int64(len(segContent)), completeBody.Size)
	assert.Equal(t, "vol-123", completeBody.VolumeID)
}

func TestCloudSegmentUploaderIncludesLeaseNonce(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	var completeBody logSegmentCompleteRequest

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "seg-1",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: ts.URL + "/complete",
			})
		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST" && r.URL.Path == "/complete":
			json.NewDecoder(r.Body).Decode(&completeBody)
			w.WriteHeader(http.StatusOK)
		}
	}

	state := NewState()
	state.SetMount("disk_mount/mnt-1", &MountState{
		EntityId:   "disk_mount/mnt-1",
		VolumeId:   "vol-456",
		LeaseNonce: "lease-nonce-xyz",
	})

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "disk.0001.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, state)

	_, err := uploader.UploadSegment(context.Background(), "vol-456", segPath)
	require.NoError(t, err)

	assert.Equal(t, "lease-nonce-xyz", completeBody.LeaseNonce)
}

func TestCloudSegmentUploaderNoLeaseNonceWithoutState(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	var completeBody logSegmentCompleteRequest

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "seg-1",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: ts.URL + "/complete",
			})
		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST" && r.URL.Path == "/complete":
			json.NewDecoder(r.Body).Decode(&completeBody)
			w.WriteHeader(http.StatusOK)
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "disk.0001.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	_, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.NoError(t, err)

	assert.Equal(t, "", completeBody.LeaseNonce)
}

func TestCloudSegmentUploaderSkipsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "empty.log")
	require.NoError(t, os.WriteFile(segPath, nil, 0644))

	uploader := &CloudSegmentUploader{log: slog.Default()}

	segID, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.NoError(t, err)
	assert.Equal(t, "", segID)
}

func TestCloudSegmentUploaderFileNotFound(t *testing.T) {
	uploader := &CloudSegmentUploader{log: slog.Default()}

	_, err := uploader.UploadSegment(context.Background(), "vol-1", "/nonexistent/path.log")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}

func TestCloudSegmentUploaderUploadRequestFails(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "seg.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	_, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestCloudSegmentUploaderDataUploadFails(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "seg-1",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: "/complete",
			})
		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("expired presigned URL"))
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "seg.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	_, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestCloudSegmentUploaderCompletionFails(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "seg-1",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: ts.URL + "/complete",
			})
		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST" && r.URL.Path == "/complete":
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("duplicate segment"))
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "seg.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	_, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409")
}

func TestCloudSegmentUploaderAbsoluteCompletedURL(t *testing.T) {
	ts, h, authClient := newTestUploaderServer(t)

	var completePath string

	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/log-segments/upload"):
			json.NewEncoder(w).Encode(logSegmentUploadResponse{
				SegmentID:    "seg-1",
				UploadURL:    ts.URL + "/upload-data",
				CompletedURL: ts.URL + "/absolute/complete",
			})
		case r.Method == "PUT" && r.URL.Path == "/upload-data":
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST":
			completePath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}
	}

	tmpDir := t.TempDir()
	segPath := filepath.Join(tmpDir, "seg.log")
	require.NoError(t, os.WriteFile(segPath, []byte("data"), 0644))

	uploader := NewCloudSegmentUploader(slog.Default(), ts.URL, authClient, nil)

	_, err := uploader.UploadSegment(context.Background(), "vol-1", segPath)
	require.NoError(t, err)

	assert.Equal(t, "/absolute/complete", completePath)
}
