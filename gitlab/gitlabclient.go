/*
Copyright 2020 The Flux CD contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitlab

import (
	"context"
	"fmt"

	"github.com/fluxcd/go-git-providers/gitprovider"
	"github.com/xanzy/go-gitlab"
	"github.com/xanzy/go-gitlab/groups"
)

// gitlabClientImpl is a wrapper around *github.Client, which implements higher-level methods,
// operating on the go-github structs. Pagination is implemented for all List* methods, all returned
// objects are validated, and HTTP errors are handled/wrapped using handleHTTPError.
// This interface is also fakeable, in order to unit-test the client.
type gitlabClient interface {
	// Client returns the underlying *github.Client
	Client() *gitlab.Client

	// Group methods

	// GetGroup is a wrapper for "GET /groups/{group}".
	// This function HTTP error wrapping, and validates the server result.
	GetGroup(ctx context.Context, groupName string) (*gitlab.Group, error)
	// ListGroups is a wrapper for "GET /groups".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListGroups(ctx context.Context) ([]*gitlab.Group, error)
	// ListSubgroups is a wrapper for "GET /groups/{group}/subgroups".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListSubgroups(ctx context.Context, groupName string) ([]*gitlab.Group, error)
	// ListGroupProjects is a wrapper for "GET /groups/{group}/projects".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListGroupProjects(ctx context.Context, groupName string) ([]*gitlab.Project, error)
	// ListGroupMembers is a wrapper for "GET /groups/{group}/members".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListGroupMembers(ctx context.Context, groupName string) ([]*gitlab.GroupMember, error)

	// Project methods

	// GetProject is a wrapper for "GET /projects/{project}".
	// This function handles HTTP error wrapping, and validates the server result.
	GetProject(ctx context.Context, projectName string) (*gitlab.Project, error)
	// GetProjects is a wrapper for "GET /projects".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListProjects(ctx context.Context) ([]*gitlab.Project, error)
	// ListProjectUsers is a wrapper for "GET /projects/{project}/users".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListProjectUsers(ctx context.Context, projectName string) ([]*gitlab.ProjectUser, error)
	// ListUserProjects is a wrapper for "GET /users/{username}/projects".
	// This function handles pagination, HTTP error wrapping, and validates the server result.
	ListUserProjects(ctx context.Context, username string) ([]*gitlab.Project, error)
	// CreateProject is a wrapper for "POST /projects"
	// This function handles HTTP error wrapping, and validates the server result.
	CreateProject(ctx context.Context, req *gitlab.Project) (*gitlab.Project, error)
	// UpdateProject is a wrapper for "PUT /projects/{project}".
	// This function handles HTTP error wrapping, and validates the server result.
	UpdateProject(ctx context.Context, req *gitlab.Project) (*gitlab.Project, error)
	// DeleteProject is a wrapper for "DELETE /projects/{project}".
	// This function handles HTTP error wrapping.
	// DANGEROUS COMMAND: In order to use this, you must set destructiveActions to true.
	DeleteProject(ctx context.Context, projectName string) error

	// // Deploy key methods

	// // ListProjectKeys is a wrapper for "GET /projects/{project}/deploy_keys".
	// // This function handles pagination, HTTP error wrapping, and validates the server result.
	// ListProjectKeys(ctx context.Context, projectName string) ([]*gitlab.Key, error)
	// // CreateProjectKey is a wrapper for "POST /projects/{project}/deploy_keys".
	// // This function handles HTTP error wrapping, and validates the server result.
	// CreateProjectKey(ctx context.Context, projectName string, req *gitlab.Key) (*gitlab.Key, error)
	// // DeleteKey is a wrapper for "DELETE /projects/{project}/deploy_keys/{key_id}".
	// // This function handles HTTP error wrapping.
	// DeleteProjectKey(ctx context.Context, projectName string, id int64) error
}

// gitlabClientImpl is a wrapper around *gitlab.Client, which implements higher-level methods,
// operating on the go-gitlab structs. See the gitlabClient interface for method documentation.
// Pagination is implemented for all List* methods, all returned
// objects are validated, and HTTP errors are handled/wrapped using handleHTTPError.
type gitlabClientImpl struct {
	c                  *gitlab.Client
	destructiveActions bool
}

// gitlabClientImpl implements gitlabClient.
var _ gitlabClient = &gitlabClientImpl{}

func (c *gitlabClientImpl) Client() *gitlab.Client {
	return c.c
}

func (c *gitlabClientImpl) GetGroup(ctx context.Context, groupName string) (*gitlab.Group, error) {
	apiObj, _, err := c.c.Groups.GetGroup(groupName, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	// Validate the API object
	if err := validateGroupAPI(apiObj); err != nil {
		return nil, err
	}
	return apiObj, nil
}

func (c *gitlabClientImpl) ListGroups(ctx context.Context) ([]*gitlab.Group, error) {
	apiObjs := []*gitlab.Group{}
	opts := &groups.ListGroupsOptions{
		AllAvailable: true,
		TopLevelOnly: true,
	}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /groups
		pageObjs, resp, listErr := c.c.Groups.ListGroups(opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	// Validate the API objects
	for _, apiObj := range apiObjs {
		if err := validateGroupAPI(apiObj); err != nil {
			return nil, err
		}
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) ListSubgroups(ctx context.Context, groupName string) ([]*gitlab.Group, error) {
	var apiObjs []*gitlab.Group
	opts := &groups.ListGroupsOptions{
		AllAvailable: true,
		TopLevelOnly: false,
	}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /groups
		pageObjs, resp, listErr := c.c.Groups.ListSubgroups(groupName, opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	// Validate the API objects
	for _, apiObj := range apiObjs {
		if err := validateGroupAPI(apiObj); err != nil {
			return nil, err
		}
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) ListGroupProjects(ctx context.Context, groupName string) ([]*gitlab.Project, error) {
	var apiObjs []*gitlab.Project
	opts := &gitlab.ListGroupProjectsOptions{}
	err := allPages(&opts.ListOptions, func() (*gitlab.Response, error) {
		pageObjs, resp, listErr := c.c.Groups.ListGroupProjects(groupName, opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	return validateProjectObjects(apiObjs)
}

func validateProjectObjects(apiObjs []*gitlab.Project) ([]*gitlab.Project, error) {
	for _, apiObj := range apiObjs {
		// Make sure apiObj is valid
		if err := validateProjectAPI(apiObj); err != nil {
			return nil, err
		}
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) ListGroupMembers(ctx context.Context, groupName string) ([]*gitlab.GroupMember, error) {
	var apiObjs []*gitlab.GroupMember
	opts := &gitlab.ListGroupMembersOptions{}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /groups/{group}/members
		pageObjs, resp, listErr := c.c.Groups.ListGroupMembers(groupName, opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) GetProject(ctx context.Context, projectName string) (*gitlab.Project, error) {
	opts := &gitlab.GetProjectOptions{}
	apiObj, _, err := c.c.Projects.GetProject(projectName, opts, gitlab.WithContext(ctx))
	return validateProjectAPIResp(apiObj, err)
}

func validateProjectAPIResp(apiObj *gitlab.Project, err error) (*gitlab.Project, error) {
	// If the response contained an error, return
	if err != nil {
		return nil, err
	}
	// Make sure apiObj is valid
	if err := validateProjectAPI(apiObj); err != nil {
		return nil, err
	}
	return apiObj, nil
}

func (c *gitlabClientImpl) ListProjects(ctx context.Context) ([]*gitlab.Project, error) {
	var apiObjs []*gitlab.Project
	opts := &gitlab.ListProjectsOptions{}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /projects
		pageObjs, resp, listErr := c.c.Projects.ListProjects(opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) ListProjectUsers(ctx context.Context, projectName string) ([]*gitlab.ProjectUser, error) {
	var apiObjs []*gitlab.ProjectUser
	opts := &gitlab.ListProjectUserOptions{}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /projects/{project}/users
		pageObjs, resp, listErr := c.c.Projects.ListProjectsUsers(projectName, opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) ListUserProjects(ctx context.Context, username string) ([]*gitlab.Project, error) {
	var apiObjs []*gitlab.Project
	opts := &gitlab.ListProjectsOptions{}
	err := allPages(opts, func() (*gitlab.Response, error) {
		// GET /projects/{project}/users
		pageObjs, resp, listErr := c.c.Projects.ListUserProjects(username, opts, gitlab.WithContext(ctx))
		apiObjs = append(apiObjs, pageObjs...)
		return resp, listErr
	})
	if err != nil {
		return nil, err
	}
	return apiObjs, nil
}

func (c *gitlabClientImpl) CreateProject(ctx context.Context, req *gitlab.Project) (*gitlab.Project, error) {
	opts := &gitlab.CreateProjectOptions{}
	apiObj, _, err := c.c.Projects.CreateProject(opts, gitlab.WithContext(ctx))
	return validateProjectAPIResp(apiObj, err)
}

func (c *gitlabClientImpl) UpdateProject(ctx context.Context, req *gitlab.Project) (*gitlab.Project, error) {
	opts := &gitlab.EditProjectOptions{}
	apiObj, _, err := c.c.Projects.EditProject(req.ID, opts, gitlab.WithContext(ctx))
	return validateProjectAPIResp(apiObj, err)
}

func (c *gitlabClientImpl) DeleteProject(ctx context.Context, projectName string) error {
	// Don't allow deleting repositories if the user didn't explicitly allow dangerous API calls.
	if !c.destructiveActions {
		return fmt.Errorf("cannot delete repository: %w", gitprovider.ErrDestructiveCallDisallowed)
	}
	// DELETE /projects/{project}
	_, err := c.c.Projects.DeleteProject(projectName, gitlab.WithContext(ctx))
	return err
}