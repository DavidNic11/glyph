package integration

import (
	"net/http"
	"testing"

	"github.com/glyph/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createFolderWithLane creates a folder owned by userA containing one lane that
// has a title, a filter rule and a non-default sort config and order.
func createFolderWithLane(t *testing.T, h *Harness) (model.Page, model.Lane) {
	t.Helper()
	w := h.Do(t, "POST", "/api/v1/pages",
		map[string]interface{}{"title": "Sprint Board", "type": "folder"}, h.UserA.ID)
	require.Equal(t, http.StatusCreated, w.Code)
	folder := Decode[model.Page](t, w)

	laneBody := map[string]interface{}{
		"title": "In Progress",
		"filterSet": map[string]interface{}{
			"conjunction": "and",
			"rules": []map[string]interface{}{
				{"id": "r1", "field": "status", "operator": "eq", "value": "todo"},
			},
		},
		"sortConfig": map[string]interface{}{"mode": "manual"},
		"order":      3,
	}
	w = h.Do(t, "POST", "/api/v1/folders/"+folder.ID.String()+"/lanes", laneBody, h.UserA.ID)
	require.Equal(t, http.StatusCreated, w.Code)
	lane := Decode[model.Lane](t, w)
	require.Equal(t, "In Progress", lane.Title)
	require.Len(t, lane.FilterSet.Rules, 1)
	return folder, lane
}

func getFolderLane(t *testing.T, h *Harness, folderID string) model.Lane {
	t.Helper()
	w := h.Do(t, "GET", "/api/v1/folders/"+folderID+"/lanes", nil, h.UserA.ID)
	require.Equal(t, http.StatusOK, w.Code)
	lanes := Decode[[]model.Lane](t, w)
	require.Len(t, lanes, 1)
	return lanes[0]
}

// TestFolderLanePartialUpdate guards against a regression where
// UpdateFolderLane bound the request into a full model.Lane and assigned every
// field unconditionally. Partial payloads — which is what a rename and a
// drag-reorder actually send — zeroed the omitted fields, silently wiping lane
// titles, filter rules and sort configuration.
func TestFolderLanePartialUpdate(t *testing.T) {
	RunSpecs(t, map[string]func(t *testing.T, h *Harness){
		// A rename sends only {title}. Filters, sort and order must survive.
		"RenamePreservesFilterSortAndOrder": func(t *testing.T, h *Harness) {
			h.ResetDB(t)
			folder, lane := createFolderWithLane(t, h)

			w := h.Do(t, "PUT",
				"/api/v1/folders/"+folder.ID.String()+"/lanes/"+lane.ID.String(),
				map[string]interface{}{"title": "In Review"}, h.UserA.ID)
			require.Equal(t, http.StatusOK, w.Code)

			got := getFolderLane(t, h, folder.ID.String())
			assert.Equal(t, "In Review", got.Title)
			assert.Len(t, got.FilterSet.Rules, 1, "rename must not wipe filter rules")
			assert.Equal(t, model.SortMode("manual"), got.SortConfig.Mode, "rename must not wipe sort config")
			assert.Equal(t, 3, got.Order, "rename must not reset order")
		},

		// A drag-reorder sends only {order}. It must succeed (previously 400,
		// because Title is binding:"required" on model.Lane) and preserve the rest.
		"ReorderPreservesTitleFilterAndSort": func(t *testing.T, h *Harness) {
			h.ResetDB(t)
			folder, lane := createFolderWithLane(t, h)

			w := h.Do(t, "PUT",
				"/api/v1/folders/"+folder.ID.String()+"/lanes/"+lane.ID.String(),
				map[string]interface{}{"order": 0}, h.UserA.ID)
			require.Equal(t, http.StatusOK, w.Code, "reorder-only payload must be accepted")

			got := getFolderLane(t, h, folder.ID.String())
			assert.Equal(t, 0, got.Order)
			assert.Equal(t, "In Progress", got.Title, "reorder must not wipe title")
			assert.Len(t, got.FilterSet.Rules, 1, "reorder must not wipe filter rules")
			assert.Equal(t, model.SortMode("manual"), got.SortConfig.Mode, "reorder must not wipe sort config")
		},

		// A full payload must still replace every field as before.
		"FullUpdateStillReplacesAllFields": func(t *testing.T, h *Harness) {
			h.ResetDB(t)
			folder, lane := createFolderWithLane(t, h)

			w := h.Do(t, "PUT",
				"/api/v1/folders/"+folder.ID.String()+"/lanes/"+lane.ID.String(),
				map[string]interface{}{
					"title": "Done",
					"filterSet": map[string]interface{}{
						"conjunction": "and",
						"rules":       []map[string]interface{}{},
					},
					"sortConfig": map[string]interface{}{"mode": "auto"},
					"order":      7,
				}, h.UserA.ID)
			require.Equal(t, http.StatusOK, w.Code)

			got := getFolderLane(t, h, folder.ID.String())
			assert.Equal(t, "Done", got.Title)
			assert.Empty(t, got.FilterSet.Rules)
			assert.Equal(t, model.SortMode("auto"), got.SortConfig.Mode)
			assert.Equal(t, 7, got.Order)
		},
	})
}
