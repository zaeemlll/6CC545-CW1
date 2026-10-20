package httpapi

import (
	"net/http"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/rpcfiles"
	"example.edu/6cc545/cw1/internal/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RemoteFiles(reg *state.Registry, nodeID string, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		peerIDs := r.URL.Query()["peer_id"]
		if len(peerIDs) != 1 || !config.ValidID(peerIDs[0]) {
			ErrorJSON(w, http.StatusBadRequest, "invalid_peer_id")
			return
		}

		peer, err := reg.Get(peerIDs[0])
		if err != nil {
			ErrorJSON(w, http.StatusNotFound, "peer_not_found")
			return
		}

		result, err := rpcfiles.Fetch(r.Context(), peer.GRPCAddress, nodeID, timeout)
		if err != nil {
			switch status.Code(err) {
			case codes.InvalidArgument:
				ErrorJSON(w, http.StatusBadRequest, "invalid_remote_request")
			case codes.Unavailable:
				ErrorJSON(w, http.StatusServiceUnavailable, "peer_unavailable")
			case codes.DeadlineExceeded:
				ErrorJSON(w, http.StatusGatewayTimeout, "peer_timeout")
			case codes.Canceled:
				ErrorJSON(w, http.StatusGatewayTimeout, "request_cancelled")
			default:
				ErrorJSON(w, http.StatusBadGateway, "remote_request_failed")
			}
			return
		}

		files := make([]model.FileMetadata, 0, len(result.GetFiles()))
		for _, f := range result.GetFiles() {
			if f == nil {
				continue
			}
			files = append(files, model.FileMetadata{
				Name:         f.GetName(),
				Size:         f.GetSize(),
				SHA256:       f.GetSha256(),
				ModifiedUnix: f.GetModifiedUnix(),
			})
		}

		WriteJSON(w, http.StatusOK, struct {
			NodeID string               `json:"node_id"`
			Files  []model.FileMetadata `json:"files"`
		}{
			NodeID: result.GetNodeId(),
			Files:  files,
		})
	}
}
