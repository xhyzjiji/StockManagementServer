package server

import (
	"net/http"

	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/store"
)

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	list, err := s.store.ListItems(r.Context(), uid(r), id, store.ItemQuery{
		CategoryID: int64(queryInt(r, "category_id", 0)),
		Keyword:    q.Get("q"),
		ByTag:      q.Get("by") == "tag",
		Sort:       q.Get("sort"),
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	d, err := s.store.GetItemDetail(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var in model.ItemInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	id, err := s.store.SaveItem(r.Context(), uid(r), nil, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in model.ItemInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if _, err := s.store.SaveItem(r.Context(), uid(r), &id, in); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (s *Server) handleDeleteItems(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(w, r, &req); err != nil {
		fail(w, r, err)
		return
	}
	n, err := s.store.SoftDeleteItems(r.Context(), uid(r), id, req.IDs)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

func (s *Server) handleListDeletedItems(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	list, err := s.store.ListDeletedItems(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleRestoreItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.RestoreItem(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDestroyItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.DestroyItem(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleApplyStock(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in model.StockInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	rec, err := s.store.ApplyStock(r.Context(), uid(r), id, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleItemRecords(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	page, err := s.store.ListItemRecords(r.Context(), uid(r), id, queryInt(r, "offset", 0), queryInt(r, "limit", 20))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleItemPrices(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	points, err := s.store.PricePoints(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}
