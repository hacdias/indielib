package micropub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("invalid request")
	ErrNotImplemented = errors.New("not implemented")
)

// Configuration is the configuration of a [Router]. Use the different [Option]
// to customize your endpoint.
type Configuration struct {
	MediaEndpoint  string
	GetSyndicateTo func() []Syndication
	GetChannels    func() []Channel
	GetCategories  func() []string
	GetPostTypes   func() []PostType
	GetVisibility  func() []string
}

// PostType is used to provide information regarding the server's [supported vocabulary].
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

type Option func(*Configuration)

// WithMediaEndpoint configures the URL of the [media endpoint]. This is used
// when a Micropub client asks for the configuration of the endpoint. If this
// is not set, a client won't be able to recognize the endpoint.
//
// [media endpoint]: https://micropub.spec.indieweb.org/#media-endpoint
func WithMediaEndpoint(endpoint string) Option {
	return func(conf *Configuration) {
		conf.MediaEndpoint = endpoint
	}
}

// WithGetSyndicateTo configures the getter for syndication targets. This allows
// for dynamic syndication targets. Return an empty slice if there are no targets.
func WithGetSyndicateTo(getSyndicateTo func() []Syndication) Option {
	return func(conf *Configuration) {
		conf.GetSyndicateTo = getSyndicateTo
	}
}

// WithGetChannels configures the getter for channels. This allows for dynamic
// channels. Return an empty slice if there are no channels.
func WithGetChannels(getChannels func() []Channel) Option {
	return func(conf *Configuration) {
		conf.GetChannels = getChannels
	}
}

// WithGetCategories configures the getter for the categories. This allows for
// dynamic categories. Return an empty slice if there are no categories.
func WithGetCategories(getCategories func() []string) Option {
	return func(conf *Configuration) {
		conf.GetCategories = getCategories
	}
}

// WithGetPostTypes configures the getter for the allowed post types. This allows
// for dynamic post types. Return an empty slice if you don't want it to be set
// in the configuration.
func WithGetPostTypes(getPostTypes func() []PostType) Option {
	return func(conf *Configuration) {
		conf.GetPostTypes = getPostTypes
	}
}

// WithGetVisibility configures the getter for supported [visibility]. Return an
// empty slice if there are no channels.
//
// [visibility]: https://indieweb.org/Micropub-extensions#Visibility
func WithGetVisibility(getVisibility func() []string) Option {
	return func(conf *Configuration) {
		conf.GetVisibility = getVisibility
	}
}

// Implementation is the backend implementation necessary to run a Micropub
// server with [Router].
//
// You must implement [Implementation.HasScope]. The remaining functions
// can return [ErrNotImplemented] if you don't support the feature.
//
// You can also return [ErrNotFound] and [ErrBadRequest] and the status
// code, as well as JSON error, will be set accordingly.
type Implementation interface {
	// HasScope returns whether or not the request is authorized for a certain scope.
	HasScope(r *http.Request, scope string) bool

	// Source returns the Microformats source of a certain URL.
	Source(url string) (map[string]any, error)

	// Source all returns the Microformats source for a [limit] amount of posts,
	// offset by the given [offset]. Used to implement [post list]. Limit will be
	// -1 by default, and offset 0.
	//
	// [post list]: https://indieweb.org/Micropub-extensions#Query_for_Post_List
	SourceMany(limit, offset int) ([]map[string]any, error)

	// Create makes a create request according to the given [Request].
	// Must return the location (e.g., URL) of the created post.
	Create(req *Request) (string, error)

	// Update makes an update request according to the given [Request].
	// Must return the location (e.g., URL) of the update post.
	Update(req *Request) (string, error)

	// Delete deletes the post at the given URL.
	Delete(url string) error

	// Undelete reverts a deletion of the post at the given URL.
	Undelete(url string) error
}

type handler struct {
	conf Configuration
	impl Implementation
}

// NewHandler creates a new Micropub [http.Handler] conforming to the [specification].
// It uses the given [RouterImplementation] and [Option]s to handle the requests.
//
// The returned handler can be mounted under the path for a Micropub server. The
// following routes are processed (assuming is mounted under /micropub):
//
//   - GET /micropub?q=source
//   - GET /micropub?q=config
//   - GET /micropub?q=syndicate-to
//   - GET /micropub?q=category
//   - GET /micropub?q=channel
//   - POST /micropub (form-encoded): create, delete, undelete
//   - POST /micropub (json): create, update, delete, undelete
//
// [specification]: https://micropub.spec.indieweb.org/
func NewHandler(impl Implementation, options ...Option) http.Handler {
	conf := &Configuration{
		MediaEndpoint:  "",
		GetSyndicateTo: func() []Syndication { return nil },
		GetChannels:    func() []Channel { return nil },
		GetCategories:  func() []string { return nil },
		GetPostTypes:   func() []PostType { return nil },
		GetVisibility:  func() []string { return nil },
	}

	for _, opt := range options {
		opt(conf)
	}

	return &handler{
		conf: *conf,
		impl: impl,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.micropubGet(w, r)
	case http.MethodPost:
		h.micropubPost(w, r)
	default:
		serveError(w, ErrNotImplemented)
	}
}

func (h *handler) micropubGet(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "source":
		h.micropubSource(w, r)
	case "config":
		config := map[string]any{}
		if h.conf.MediaEndpoint != "" {
			config["media-endpoint"] = h.conf.MediaEndpoint
		}
		if syndicateTo := h.conf.GetSyndicateTo(); len(syndicateTo) != 0 {
			config["syndicate-to"] = syndicateTo
		}
		if channels := h.conf.GetChannels(); len(channels) != 0 {
			config["channels"] = channels
		}
		if categories := h.conf.GetCategories(); len(categories) != 0 {
			config["categories"] = categories
		}
		if postTypes := h.conf.GetPostTypes(); len(postTypes) != 0 {
			config["post-types"] = postTypes
		}
		if visibility := h.conf.GetVisibility(); len(visibility) != 0 {
			config["visibility"] = visibility
		}
		serveJSON(w, http.StatusOK, config)
	case "syndicate-to":
		syndicateTo := h.conf.GetSyndicateTo()
		if len(syndicateTo) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"syndicate-to": syndicateTo})
		}
	case "category":
		categories := h.conf.GetCategories()
		if len(categories) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"categories": categories})
		}
	case "channel":
		channels := h.conf.GetChannels()
		if len(channels) == 0 {
			serveError(w, ErrNotFound)
		} else {
			serveJSON(w, http.StatusOK, map[string]any{"channels": channels})
		}
	default:
		serveError(w, ErrNotFound)
	}
}

func (h *handler) micropubSource(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		limitStr := r.URL.Query().Get("limit")
		if limitStr == "" {
			limitStr = "-1"
		}

		offsetStr := r.URL.Query().Get("offset")
		if offsetStr == "" {
			offsetStr = "0"
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			serveError(w, errors.Join(ErrBadRequest, err))
			return
		}

		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			serveError(w, errors.Join(ErrBadRequest, err))
			return
		}

		items, err := h.impl.SourceMany(limit, offset)
		if err != nil {
			serveError(w, err)
			return
		}

		serveJSON(w, http.StatusOK, map[string]any{
			"items": items,
		})
		return
	}

	item, err := h.impl.Source(url)
	if err != nil {
		serveError(w, err)
		return
	}

	serveJSON(w, http.StatusOK, item)
}

func (h *handler) micropubPost(w http.ResponseWriter, r *http.Request) {
	mr, err := ParseRequest(r)
	if err != nil {
		serveError(w, errors.Join(ErrBadRequest, err))
		return
	}

	switch mr.Action {
	case ActionCreate:
		if !h.checkScope(w, r, "create") {
			return
		}
		location, err := h.impl.Create(mr)
		if err != nil {
			serveError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		http.Redirect(w, r, location, http.StatusAccepted)
	case ActionUpdate:
		if !h.checkScope(w, r, "update") {
			return
		}
		location, err := h.impl.Update(mr)
		if err != nil {
			serveError(w, err)
			return
		}
		http.Redirect(w, r, location, http.StatusOK)
	case ActionDelete:
		if !h.checkScope(w, r, "delete") {
			return
		}
		err = h.impl.Delete(mr.URL)
		if err != nil {
			serveError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case ActionUndelete:
		if !h.checkScope(w, r, "undelete") {
			return
		}
		err = h.impl.Undelete(mr.URL)
		if err != nil {
			serveError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		serveError(w, fmt.Errorf("%w: invalid action '%q'", ErrBadRequest, mr.Action))
	}
}

func (h *handler) checkScope(w http.ResponseWriter, r *http.Request, scope string) bool {
	if !h.impl.HasScope(r, scope) {
		serveErrorJSON(w, http.StatusForbidden, "insufficient_scope", "Insufficient scope.")
		return false
	}

	return true
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
