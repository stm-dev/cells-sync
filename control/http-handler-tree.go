/*
 * Copyright 2019 Abstrium SAS
 *
 *  This file is part of Cells Sync.
 *
 *  Cells Sync is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  Cells Sync is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with Cells Sync.  If not, see <https://www.gnu.org/licenses/>.
 */

package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"

	"github.com/pydio/cells-sync/endpoint"
	"github.com/pydio/cells/common"
	"github.com/pydio/cells/common/log"
	"github.com/pydio/cells/common/proto/tree"
	"github.com/pydio/cells/common/sync/model"
)

type TreeRequest struct {
	EndpointURI      string
	Path             string
	windowsDrive     string
	browseWinVolumes bool
	endpoint         model.Endpoint
}

// TreeResponse is a fake protobuf used for marshaling responses to tree requests.
type TreeResponse struct {
	Node     *tree.Node
	Children []*tree.Node
}

// ProtoMessage implements Proto() interface
func (l *TreeResponse) ProtoMessage() {}

// Reset implements Proto() interface
func (l *TreeResponse) Reset() {}

// String implements Proto() interface
func (l *TreeResponse) String() string {
	return ""
}

// MarshalJSON uses jsonpb for marshaling struct to JSON
func (l *TreeResponse) MarshalJSON() ([]byte, error) {
	encoder := jsonpb.Marshaler{}
	buffer := bytes.NewBuffer(nil)
	e := encoder.Marshal(buffer, l)
	return buffer.Bytes(), e
}

func (h *HttpServer) writeError(i *gin.Context, e error) {
	log.Logger(h.ctx).Error("Request error :" + e.Error())
	data := map[string]string{
		"error": e.Error(),
	}
	if c := errors.Cause(e); c != e {
		data["stack"] = fmt.Sprintf("%+v\n", e)
	}
	i.JSON(http.StatusInternalServerError, data)
}

func (h *HttpServer) applyWindowsTransformation(request *TreeRequest) error {
	u, e := url.Parse(request.EndpointURI)
	if e != nil {
		return e
	}

	if u.Path == "" {
		pathLen := len(request.Path)
		if pathLen > 2 {
			prefix := "/" + strings.ToUpper(request.Path[1:3])
			request.windowsDrive = prefix + "/"

			builtPath := ""
			if pathLen > 3 {
				request.Path = strings.Replace(builtPath+request.Path[3:], "\\", "/", -1)
			} else {
				request.Path = "\\"
			}

			request.EndpointURI = request.EndpointURI + prefix
		} else {
			request.Path = "/"
			request.browseWinVolumes = true
		}
	} else {
		request.browseWinVolumes = false
	}

	return nil
}

func (h *HttpServer) parseTreeRequest(c *gin.Context) (*TreeRequest, error) {
	var request TreeRequest
	dec := json.NewDecoder(c.Request.Body)
	if e := dec.Decode(&request); e != nil {
		return nil, e
	}

	// Special case for browsing windows FS
	if strings.HasPrefix(request.EndpointURI, "fs://") && runtime.GOOS == "windows" {
		err := h.applyWindowsTransformation(&request)
		if err != nil {
			return &request, err
		}
	}

	ep, e := endpoint.EndpointFromURI(request.EndpointURI, "", true)
	if e != nil {
		return nil, e
	}
	request.endpoint = ep
	return &request, nil
}

func (h *HttpServer) ls(c *gin.Context) {
	request, e := h.parseTreeRequest(c)
	if e != nil {
		h.writeError(c, e)
		return
	}

	log.Logger(h.ctx).Info("Browsing " + request.endpoint.GetEndpointInfo().URI + " on path " + request.Path)

	response := &TreeResponse{}

	if request.browseWinVolumes {
		response.Node = &tree.Node{}
		for _, v := range browseWinVolumes(h.ctx) {
			response.Children = append(response.Children, v)
		}
	} else if node, err := request.endpoint.LoadNode(h.ctx, request.Path); err == nil {
		response.Node = node.WithoutReservedMetas()
		if !node.IsLeaf() {
			if source, ok := model.AsPathSyncSource(request.endpoint); ok {
				source.Walk(func(p string, node *tree.Node, err error) {
					if err != nil {
						return
					}
					if request.windowsDrive != "" {
						p = path.Join(request.windowsDrive, p)
						node.Path = p
					}
					// Small fix for router case at level 0
					if strings.HasPrefix(node.Uuid, "DATASOURCE:") {
						node.Type = tree.NodeType_COLLECTION
					}
					if path.Base(p) != common.PYDIO_SYNC_HIDDEN_FILE_META && !strings.HasPrefix(path.Base(p), ".") {
						response.Children = append(response.Children, node.WithoutReservedMetas())
					}
				}, request.Path, false)
			}
		}
	} else {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *HttpServer) mkdir(c *gin.Context) {
	request, e := h.parseTreeRequest(c)
	if e != nil {
		h.writeError(c, e)
		return
	}
	target, ok := model.AsPathSyncTarget(request.endpoint)
	if !ok {
		h.writeError(c, fmt.Errorf("cannot.write"))
		return
	}
	newNode := &tree.Node{
		Path: request.Path,
		Type: tree.NodeType_COLLECTION,
	}
	if e := target.CreateNode(context.Background(), newNode, false); e != nil {
		h.writeError(c, e)
		return
	}
	// Special case for cells : block until folder is correctly indexed
	if strings.HasPrefix(target.GetEndpointInfo().URI, "http") {
		model.Retry(func() error {
			_, e := target.LoadNode(context.Background(), newNode.Path)
			return e
		}, 2*time.Second, 10*time.Second)
	}

	log.Logger(context.Background()).Info("Created folder on " + request.endpoint.GetEndpointInfo().URI + " at path " + request.Path)
	c.JSON(http.StatusOK, &TreeResponse{Node: newNode})
}

func (h *HttpServer) defaultDir(c *gin.Context) {
	request, e := h.parseTreeRequest(c)
	if e != nil {
		h.writeError(c, e)
		return
	}
	outputDir := endpoint.DefaultDirForURI(request.EndpointURI)
	epDir := outputDir
	if outputDir == "" {
		c.JSON(http.StatusOK, &TreeResponse{Node: &tree.Node{Path: ""}})
		return
	}
	if runtime.GOOS == "windows" {
		outputDir = "/" + outputDir
		request.Path = outputDir
		if er := h.applyWindowsTransformation(request); er != nil {
			c.JSON(http.StatusOK, &TreeResponse{Node: &tree.Node{Path: ""}})
			return
		}
		epDir = request.Path
	}
	if node, err := request.endpoint.LoadNode(context.Background(), epDir); err == nil {
		node.Path = outputDir
		c.JSON(http.StatusOK, &TreeResponse{Node: node})
		return
	}

	target, ok := model.AsPathSyncTarget(request.endpoint)
	if !ok {
		h.writeError(c, fmt.Errorf("cannot.write"))
		return
	}
	if e := target.CreateNode(context.Background(), &tree.Node{Path: epDir, Type: tree.NodeType_COLLECTION}, false); e != nil {
		h.writeError(c, e)
		return
	}
	c.JSON(http.StatusOK, &TreeResponse{Node: &tree.Node{Path: outputDir, Type: tree.NodeType_COLLECTION}})
}
