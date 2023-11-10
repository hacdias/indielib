package micropub

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("invalid request")
	ErrNotImplemented = errors.New("not implemented")
)

// RouterImplementation is the backend implementation necessary to run a Micropub
// server with [Router].
//
// You must implement [RouterImplementation.HasScope]. The remaining functions
// can return [ErrNotImplemented] if you don't support the feature.
//
// You can also return [ErrNotFound] and [ErrBadRequest] and the status
// code, as well as JSON error, will be set accordingly.
type RouterImplementation interface {
	// HasScope returns whether or not the request is authorized for a certain scope.
	HasScope(r *http.Request, scope string) bool

	// UploadMedia uploads the given file with the given header. Must return the
	// location (e.g., URL) of the uploaded file.
	UploadMedia(file multipart.File, header *multipart.FileHeader) (string, error)

	// Source returns the Microformats source of a certain URL.
	Source(url string) (map[string]any, error)

	// Create makes a create request according to the given [Request].
	// Must return the location (e.g., URL) of the created post.
	Create(req *Request) (string, error)

	// Update makes an update request according to the given [Request].
	// Must return the location (e.g., URL) of the update post.
	Update(req *Request) (string, error)

	// Delete deletes the post at the given URL.
	Delete(url string) error

	// Undelete undeletes the post at the given URL.
	Undelete(url string) error
}

// RouterConfiguration is the interface you have to implement to provide configuration.
// This is necessary for the micropub endpoint config query, and related.
type RouterConfiguration interface {
	// MediaEndpoint is the URL of the [media endpoint]. Return empty string to not set.
	//
	// [media endpoint]: https://micropub.spec.indieweb.org/#media-endpoint
	MediaEndpoint() string

	// SyndicateTo are the syndication targets. Return empty slice to not set.
	SyndicateTo() []Syndication

	// Channels are the available channels. Return empty slice to not set.
	Channels() []Channel

	// Categories are the available categories. Return empty slice to not set.
	Categories() []string

	// PostTypes are the allowed post types. Return empty slice to not set.
	PostTypes() []PostType
}

// PostType is part of [MicropubConfig] and used to provide information regarding
// this server's [supported vocabulary].
//
// [supported vocabulary]: https://indieweb.org/Micropub-extensions#Query_for_Supported_Vocabulary
type PostType struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Properties []string `json:"properties,omitempty"`
	Required   []string `json:"required-properties,omitempty"`
}

type uidAndName struct {
	UID  string `json:"uid"`
	Name string `json:"name,omitempty"`
}

// Syndication represents a syndication target.
type Syndication = uidAndName

// Channel represents a channel.
type Channel = uidAndName

type Router struct {
	impl RouterImplementation
	conf RouterConfiguration
}

// NewRouter creates a new [Router] object with the given [RouterImplementation]
// and the given [MicropubConfig]. The configuration is used to return to a Micropub
// client when it requests for configuration.
func NewRouter(impl RouterImplementation, conf RouterConfiguration) *Router {
	return &Router{
		impl: impl,
		conf: conf,
	}
}

// MicropubHandler is an [http.HandlerFunc] that can be mounted under the path
// for a micropub server handler. The following routes are processed:
//
//		GET /micropub?q=source
//		GET /micropub?q=config
//		GET /micropub?q=syndicate-to
//		GET /micropub?q=category
//		GET /micropub?q=channel
//	 	POST /micropub
//			- as form-encoded: create, delete, undelete
//			- as json: create, update, delete, undelete
//
// The provided [RouterImplementation] and [RouterConfiguration] are used to
// fulfill the requests according to the [specification].
//
// [specification]: https://micropub.spec.indieweb.org/
func (ro *Router) MicropubHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ro.micropubGet(w, r)
	case http.MethodPost:
		ro.micropubPost(w, r)
	default:
		serveError(w, ErrNotImplemented)
	}
}

func (ro *Router) micropubGet(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "source":
		ro.micropubSource(w, r)
	case "config":
		config := map[string]any{}
		if mediaEndpoint := ro.conf.MediaEndpoint(); mediaEndpoint != "" {
			config["media-endpoint"] = mediaEndpoint
		}
		if syndicateTo := ro.conf.SyndicateTo(); len(syndicateTo) != 0 {
			config["syndicate-to"] = syndicateTo
		}
		if channels := ro.conf.Channels(); len(channels) != 0 {
			config["channels"] = channels
		}
		if categories := ro.conf.Categories(); len(categories) != 0 {
			config["categories"] = categories
		}
		if postTypes := ro.conf.PostTypes(); len(postTypes) != 0 {
			config["post-types"] = postTypes
		}
		serveJSON(w, http.StatusOK, config)
	case "syndicate-to":
		syndicateTo := ro.conf.SyndicateTo()
		if len(syndicateTo) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"syndicate-to": syndicateTo})
		}
	case "category":
		categories := ro.conf.Categories()
		if len(categories) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"categories": categories})
		}
	case "channel":
		channels := ro.conf.Channels()
		if len(channels) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"channels": channels})
		}
	default:
		serveError(w, ErrNotFound)
	}
}

func (ro *Router) micropubSource(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		serveError(w, fmt.Errorf("%w: request is missing 'url' query parameter", ErrBadRequest))
		return
	}

	obj, err := ro.impl.Source(url)
	if err != nil {
		serveError(w, err)
		return
	}

	serveJSON(w, http.StatusOK, obj)
}

func (ro *Router) micropubPost(w http.ResponseWriter, r *http.Request) {
	mr, err := ParseRequest(r)
	if err != nil {
		serveError(w, errors.Join(ErrBadRequest, err))
		return
	}

	switch mr.Action {
	case ActionCreate:
		if !ro.checkScope(w, r, "create") {
			return
		}
		location, err := ro.impl.Create(mr)
		if err != nil {
			serveError(w, err)
			return
		}
		http.Redirect(w, r, location, http.StatusAccepted)
	case ActionUpdate:
		if !ro.checkScope(w, r, "update") {
			return
		}
		location, err := ro.impl.Update(mr)
		if err != nil {
			serveError(w, err)
			return
		}
		http.Redirect(w, r, location, http.StatusOK)
	case ActionDelete:
		if !ro.checkScope(w, r, "delete") {
			return
		}
		err = ro.impl.Delete(mr.URL)
		if err != nil {
			serveError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case ActionUndelete:
		if !ro.checkScope(w, r, "undelete") {
			return
		}
		err = ro.impl.Undelete(mr.URL)
		if err != nil {
			serveError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		serveError(w, fmt.Errorf("%w: invalid action '%q'", ErrBadRequest, mr.Action))
	}
}

// MicropubMediaHandler is an [http.HandlerFunc] that can be mounted under
// the path for a micropub media server handler.
func (ro *Router) MicropubMediaHandler(w http.ResponseWriter, r *http.Request) {
	if !ro.checkScope(w, r, "media") {
		return
	}

	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		serveError(w, fmt.Errorf("%w: file is too large", ErrBadRequest))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		serveError(w, errors.Join(ErrBadRequest, err))
		return
	}
	defer file.Close()

	redirect, err := ro.impl.UploadMedia(file, header)
	if err != nil {
		serveError(w, err)
		return
	}

	http.Redirect(w, r, redirect, http.StatusCreated)
}

func serveError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		serveErrorJSON(w, http.StatusNotFound, "invalid_request", err.Error())
	} else if errors.Is(err, ErrBadRequest) {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
	} else if errors.Is(err, ErrNotImplemented) {
		serveErrorJSON(w, http.StatusNotImplemented, "invalid_request", err.Error())
	} else {
		serveErrorJSON(w, http.StatusInternalServerError, "server_error", err.Error())
	}
}

func serveJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func serveErrorJSON(w http.ResponseWriter, code int, err, errDescription string) {
	serveJSON(w, code, map[string]string{
		"error":             err,
		"error_description": errDescription,
	})
}

func (ro *Router) checkScope(w http.ResponseWriter, r *http.Request, scope string) bool {
	if !ro.impl.HasScope(r, scope) {
		serveErrorJSON(w, http.StatusForbidden, "insufficient_scope", "Insufficient scope.")
		return false
	}

	return true
}
