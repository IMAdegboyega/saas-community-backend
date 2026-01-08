package posts

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

	// Post CRUD
	api.HandleFunc("/posts", handler.CreatePost).Methods("POST")
	api.HandleFunc("/posts/{id}", handler.GetPost).Methods("GET")
	api.HandleFunc("/posts/{id}", handler.UpdatePost).Methods("PUT")
	api.HandleFunc("/posts/{id}", handler.DeletePost).Methods("DELETE")

	// Feed
	api.HandleFunc("/feed", handler.GetFeed).Methods("GET")
	api.HandleFunc("/users/{id}/posts", handler.GetUserPosts).Methods("GET")

	// Interactions
	api.HandleFunc("/posts/{id}/like", handler.LikePost).Methods("POST")
	api.HandleFunc("/posts/{id}/unlike", handler.UnlikePost).Methods("POST")
	api.HandleFunc("/posts/{id}/save", handler.SavePost).Methods("POST")
	api.HandleFunc("/posts/{id}/unsave", handler.UnsavePost).Methods("POST")
	api.HandleFunc("/posts/saved", handler.GetSavedPosts).Methods("GET")

	// Comments
	api.HandleFunc("/posts/{id}/comments", handler.CreateComment).Methods("POST")
	api.HandleFunc("/posts/{id}/comments", handler.GetPostComments).Methods("GET")
	api.HandleFunc("/comments/{id}", handler.DeleteComment).Methods("DELETE")
}

func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req CreatePostRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	post, err := h.service.CreatePost(r.Context(), userID, &req)
	if err != nil {
		common.InternalError(w, "Failed to create post")
		return
	}

	common.Created(w, "Post created successfully", post)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	post, err := h.service.GetPost(r.Context(), postID, userID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			common.NotFound(w, "Post not found")
			return
		}
		common.InternalError(w, "Failed to get post")
		return
	}

	common.Success(w, "", post)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	var req UpdatePostRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	post, err := h.service.UpdatePost(r.Context(), userID, postID, &req)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			common.NotFound(w, "Post not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized to update this post")
			return
		}
		common.InternalError(w, "Failed to update post")
		return
	}

	common.Success(w, "Post updated", post)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	if err := h.service.DeletePost(r.Context(), userID, postID); err != nil {
		if errors.Is(err, ErrPostNotFound) {
			common.NotFound(w, "Post not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized to delete this post")
			return
		}
		common.InternalError(w, "Failed to delete post")
		return
	}

	common.Success(w, "Post deleted", nil)
}

func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	feedType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	posts, err := h.service.GetFeed(r.Context(), userID, feedType, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get feed")
		return
	}

	common.Success(w, "", posts)
}

func (h *Handler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
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

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	posts, total, err := h.service.GetUserPosts(r.Context(), userID, currentUserID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get posts")
		return
	}

	common.SuccessWithMeta(w, "", posts, &common.Meta{Total: total})
}

func (h *Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	if err := h.service.LikePost(r.Context(), userID, postID); err != nil {
		if errors.Is(err, ErrPostNotFound) {
			common.NotFound(w, "Post not found")
			return
		}
		common.InternalError(w, "Failed to like post")
		return
	}

	common.Success(w, "Post liked", nil)
}

func (h *Handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	if err := h.service.UnlikePost(r.Context(), userID, postID); err != nil {
		common.InternalError(w, "Failed to unlike post")
		return
	}

	common.Success(w, "Post unliked", nil)
}

func (h *Handler) SavePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	if err := h.service.SavePost(r.Context(), userID, postID); err != nil {
		common.InternalError(w, "Failed to save post")
		return
	}

	common.Success(w, "Post saved", nil)
}

func (h *Handler) UnsavePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	if err := h.service.UnsavePost(r.Context(), userID, postID); err != nil {
		common.InternalError(w, "Failed to unsave post")
		return
	}

	common.Success(w, "Post unsaved", nil)
}

func (h *Handler) GetSavedPosts(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	posts, total, err := h.service.GetSavedPosts(r.Context(), userID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get saved posts")
		return
	}

	common.SuccessWithMeta(w, "", posts, &common.Meta{Total: total})
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	var req CreateCommentRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	comment, err := h.service.CreateComment(r.Context(), userID, postID, &req)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			common.NotFound(w, "Post not found")
			return
		}
		common.InternalError(w, "Failed to create comment")
		return
	}

	common.Created(w, "Comment created", comment)
}

func (h *Handler) GetPostComments(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	postID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid post ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	comments, total, err := h.service.GetPostComments(r.Context(), postID, userID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get comments")
		return
	}

	common.SuccessWithMeta(w, "", comments, &common.Meta{Total: total})
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	commentID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid comment ID")
		return
	}

	if err := h.service.DeleteComment(r.Context(), userID, commentID); err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			common.NotFound(w, "Comment not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			common.Forbidden(w, "Not authorized to delete this comment")
			return
		}
		common.InternalError(w, "Failed to delete comment")
		return
	}

	common.Success(w, "Comment deleted", nil)
}
