package server

import (
	"net/http"

	"private/construction_warehouse/internal/model"
)

func (s *Server) handleListWorks(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWorks(r.Context(), uid(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateWork(w http.ResponseWriter, r *http.Request) {
	var in model.WorkInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	id, err := s.store.CreateWork(r.Context(), uid(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (s *Server) handleGetWork(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	d, err := s.store.GetWorkDetail(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateWork(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in model.WorkUpdateInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.UpdateWork(r.Context(), uid(r), id, in); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDeleteWork(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.DeleteWork(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleAppendWorkBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in model.WorkBatchInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.AppendWorkBatch(r.Context(), uid(r), id, in); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleListMaterials(w http.ResponseWriter, r *http.Request) {
	whID := int64(queryInt(r, "warehouse_id", 0))
	list, err := s.store.ListMaterials(r.Context(), uid(r), whID, r.URL.Query().Get("q"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}
