package stories

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tommygebru/kiekky-backend/internal/common"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware func(http.Handler) http.Handler) {
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(authMiddleware)

	// Stories
	api.HandleFunc("/stories", handler.CreateStory).Methods("POST")
	api.HandleFunc("/stories/feed", handler.GetFeedStories).Methods("GET")
	api.HandleFunc("/stories/{id}", handler.GetStory).Methods("GET")
	api.HandleFunc("/stories/{id}", handler.DeleteStory).Methods("DELETE")
	api.HandleFunc("/stories/{id}/view", handler.ViewStory).Methods("POST")
	api.HandleFunc("/stories/{id}/viewers", handler.GetStoryViewers).Methods("GET")
	api.HandleFunc("/users/{id}/stories", handler.GetUserStories).Methods("GET")

	// Highlights
	api.HandleFunc("/highlights", handler.CreateHighlight).Methods("POST")
	api.HandleFunc("/highlights/{id}", handler.DeleteHighlight).Methods("DELETE")
	api.HandleFunc("/highlights/{id}/stories", handler.AddToHighlight).Methods("POST")
	api.HandleFunc("/users/{id}/highlights", handler.GetUserHighlights).Methods("GET")
}

func (h *Handler) CreateStory(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req CreateStoryRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	story, err := h.service.CreateStory(r.Context(), userID, &req)
	if err != nil {
		common.InternalError(w, "Failed to create story")
		return
	}

	common.Created(w, "Story created", story)
}

func (h *Handler) GetStory(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	storyID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid story ID")
		return
	}

	story, err := h.service.GetStory(r.Context(), storyID, userID)
	if err != nil {
		if errors.Is(err, ErrStoryNotFound) || errors.Is(err, ErrStoryExpired) {
			common.NotFound(w, "Story not found")
			return
		}
		common.InternalError(w, "Failed to get story")
		return
	}

	common.Success(w, "", story)
}

func (h *Handler) DeleteStory(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	storyID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid story ID")
		return
	}

	if err := h.service.DeleteStory(r.Context(), userID, storyID); err != nil {
		if errors.Is(err, ErrStoryNotFound) {
			common.NotFound(w, "Story not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized to delete this story")
			return
		}
		common.InternalError(w, "Failed to delete story")
		return
	}

	common.Success(w, "Story deleted", nil)
}

func (h *Handler) GetUserStories(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	userID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	stories, err := h.service.GetUserStories(r.Context(), userID, currentUserID)
	if err != nil {
		common.InternalError(w, "Failed to get stories")
		return
	}

	common.Success(w, "", stories)
}

func (h *Handler) GetFeedStories(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	stories, err := h.service.GetFeedStories(r.Context(), userID)
	if err != nil {
		// Log the actual error for debugging
		println("GetFeedStories error:", err.Error())
		common.InternalError(w, "Failed to get stories feed")
		return
	}

	common.Success(w, "", stories)
}

func (h *Handler) ViewStory(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	storyID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid story ID")
		return
	}

	if err := h.service.ViewStory(r.Context(), storyID, userID); err != nil {
		if errors.Is(err, ErrStoryNotFound) || errors.Is(err, ErrStoryExpired) {
			common.NotFound(w, "Story not found")
			return
		}
		common.InternalError(w, "Failed to record view")
		return
	}

	common.Success(w, "View recorded", nil)
}

func (h *Handler) GetStoryViewers(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	storyID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid story ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	viewers, total, err := h.service.GetStoryViewers(r.Context(), userID, storyID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized to view this")
			return
		}
		common.InternalError(w, "Failed to get viewers")
		return
	}

	common.SuccessWithMeta(w, "", viewers, &common.Meta{Total: total})
}

func (h *Handler) CreateHighlight(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req CreateHighlightRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	highlight, err := h.service.CreateHighlight(r.Context(), userID, &req)
	if err != nil {
		common.InternalError(w, "Failed to create highlight")
		return
	}

	common.Created(w, "Highlight created", highlight)
}

func (h *Handler) GetUserHighlights(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	highlights, err := h.service.GetUserHighlights(r.Context(), userID)
	if err != nil {
		common.InternalError(w, "Failed to get highlights")
		return
	}

	common.Success(w, "", highlights)
}

func (h *Handler) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	highlightID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid highlight ID")
		return
	}

	if err := h.service.DeleteHighlight(r.Context(), userID, highlightID); err != nil {
		if errors.Is(err, ErrHighlightNotFound) {
			common.NotFound(w, "Highlight not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized")
			return
		}
		common.InternalError(w, "Failed to delete highlight")
		return
	}

	common.Success(w, "Highlight deleted", nil)
}

func (h *Handler) AddToHighlight(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	highlightID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid highlight ID")
		return
	}

	var req AddToHighlightRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	if err := h.service.AddToHighlight(r.Context(), userID, highlightID, &req); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized")
			return
		}
		common.InternalError(w, "Failed to add stories")
		return
	}

	common.Success(w, "Stories added to highlight", nil)
}
