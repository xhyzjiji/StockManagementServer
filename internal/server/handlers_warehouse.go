package server

import (
	"net/http"
	"strings"

	"private/construction_warehouse/internal/model"
)

type warehouseInput struct {
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

func (in *warehouseInput) validate() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Remark = strings.TrimSpace(in.Remark)
	if in.Name == "" {
		return model.Invalid("请输入仓库名称")
	}
	if len([]rune(in.Name)) > 128 {
		return model.Invalid("仓库名称过长")
	}
	if len([]rune(in.Remark)) > 1000 {
		return model.Invalid("备注过长")
	}
	return nil
}

func validateName(name *string, what string) error {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return model.Invalid("请输入" + what)
	}
	if len([]rune(*name)) > 128 {
		return model.Invalid(what + "过长")
	}
	return nil
}

func (s *Server) handleListWarehouses(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWarehouses(r.Context(), uid(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateWarehouse(w http.ResponseWriter, r *http.Request) {
	var in warehouseInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := in.validate(); err != nil {
		fail(w, r, err)
		return
	}
	id, err := s.store.CreateWarehouse(r.Context(), uid(r), in.Name, in.Remark)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (s *Server) handleGetWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	wh, err := s.store.GetWarehouse(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wh)
}

func (s *Server) handleUpdateWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in warehouseInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := in.validate(); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.UpdateWarehouse(r.Context(), uid(r), id, in.Name, in.Remark); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDeleteWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.SoftDeleteWarehouse(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleListDeletedWarehouses(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListDeletedWarehouses(r.Context(), uid(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleRestoreWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.RestoreWarehouse(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDestroyWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.DestroyWarehouse(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

// ---------------- 类目 / Categories ----------------

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	list, err := s.store.ListCategories(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type nameInput struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in nameInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := validateName(&in.Name, "类目名称"); err != nil {
		fail(w, r, err)
		return
	}
	newID, err := s.store.CreateCategory(r.Context(), uid(r), id, in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": newID})
}

func (s *Server) handleGetCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	c, err := s.store.GetCategory(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleRenameCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in nameInput
	if err := readJSON(w, r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := validateName(&in.Name, "类目名称"); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.RenameCategory(r.Context(), uid(r), id, in.Name); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.store.DeleteCategory(r.Context(), uid(r), id); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}

// ---------------- 属性定义 / Attribute definitions ----------------

func (s *Server) handleListAttrs(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	list, err := s.store.ListAttrDefs(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSaveAttrs(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var drafts []model.AttrDefDraft
	if err := readJSON(w, r, &drafts); err != nil {
		fail(w, r, err)
		return
	}
	if len(drafts) > 100 {
		fail(w, r, model.Invalid("属性数量不能超过 100 个"))
		return
	}
	if err := s.store.SaveAttrDefs(r.Context(), uid(r), id, drafts); err != nil {
		fail(w, r, err)
		return
	}
	list, err := s.store.ListAttrDefs(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ---------------- 统计 / Stats ----------------

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	st, err := s.store.Stats(r.Context(), uid(r), id, queryInt(r, "tz_offset", 0), queryInt(r, "weeks", 26))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleTagStats(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	defID := int64(queryInt(r, "def_id", 0))
	slices, err := s.store.TagDistribution(r.Context(), uid(r), id, defID, r.URL.Query().Get("group") == "item")
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, slices)
}

func (s *Server) handleWarehouseRecords(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	page, err := s.store.ListWarehouseRecords(r.Context(), uid(r), id, queryInt(r, "offset", 0), queryInt(r, "limit", 50))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
