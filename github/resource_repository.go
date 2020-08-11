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

package github

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	gitprovider "github.com/fluxcd/go-git-providers"
	"github.com/fluxcd/go-git-providers/validation"
	"github.com/google/go-github/v32/github"
)

func newUserRepository(ctx *clientContext, apiObj *github.Repository, ref gitprovider.RepositoryRef) *userRepository {
	return &userRepository{
		clientContext: ctx,
		r:             *apiObj,
		ref:           ref,
		deployKeys: &DeployKeyClient{
			clientContext: ctx,
			ref:           ref,
		},
	}
}

var _ gitprovider.UserRepository = &userRepository{}

type userRepository struct {
	*clientContext

	r   github.Repository
	ref gitprovider.RepositoryRef

	deployKeys *DeployKeyClient
}

func (r *userRepository) Get() gitprovider.RepositoryInfo {
	return repositoryFromAPI(&r.r)
}

func (r *userRepository) Set(info gitprovider.RepositoryInfo) error {
	if err := info.ValidateInfo(); err != nil {
		return err
	}
	repositoryInfoToAPIObj(&info, &r.r)
	return nil
}

func (r *userRepository) APIObject() interface{} {
	return &r.r
}

func (r *userRepository) Repository() gitprovider.RepositoryRef {
	return r.ref
}

func (r *userRepository) DeployKeys() gitprovider.DeployKeyClient {
	return r.deployKeys
}

// Update will apply the desired state in this object to the server.
// Only set fields will be respected (i.e. PATCH behaviour).
// In order to apply changes to this object, use the .Set({Resource}Info) error
// function, or cast .APIObject() to a pointer to the provider-specific type
// and set custom fields there.
//
// ErrNotFound is returned if the resource does not exist.
//
// The internal API object will be overridden with the received server data.
func (r *userRepository) Update(ctx context.Context) error {
	// PATCH /repos/{owner}/{repo}
	apiObj, _, err := r.c.Repositories.Edit(ctx, r.ref.GetIdentity(), r.ref.GetRepository(), &r.r)
	// Run through validation
	apiObj, err = validateRepositoryAPIResp(apiObj, err)
	if err != nil {
		return err
	}
	r.r = *apiObj
	return nil
}

// Reconcile makes sure the desired state in this object (called "req" here) becomes
// the actual state in the backing Git provider.
//
// If req doesn't exist under the hood, it is created (actionTaken == true).
// If req doesn't equal the actual state, the resource will be updated (actionTaken == true).
// If req is already the actual state, this is a no-op (actionTaken == false).
//
// The internal API object will be overridden with the received server data if actionTaken == true.
func (r *userRepository) Reconcile(ctx context.Context) (bool, error) {
	apiObj, err := getRepository(r.c, ctx, r.ref)
	if err != nil {
		// Create if not found
		if errors.Is(err, gitprovider.ErrNotFound) {
			orgName := ""
			if orgRef, ok := r.ref.(gitprovider.OrgRepositoryRef); ok {
				orgName = orgRef.Organization
			}
			repo, err := createRepositoryData(r.c, ctx, orgName, &r.r)
			if err != nil {
				return true, err
			}
			r.r = *repo
			return true, nil
		}

		return false, err
	}

	// If desired state already is actual, just return
	// TODO: Create an equality method for "settable" fields of the repository object
	// Comparing two API objects to each other using reflect.DeepEqual or JSON doesn't work
	// as current status is conflated in the same object and will result in race conditions.
	// For now, as a workaround, we'll just compare the RepositoryInfo with each other.
	if reflect.DeepEqual(repositoryFromAPI(apiObj), r.Get()) {
		return false, nil
	}
	// Otherwise, make the desired state the actual state
	return true, r.Update(ctx)
}

// Delete deletes the current resource irreversebly.
//
// ErrNotFound is returned if the resource doesn't exist anymore.
func (r *userRepository) Delete(ctx context.Context) error {
	// Don't allow deleting repositories if the user didn't explicitely allow dangerous API calls.
	if !r.destructiveActions {
		return fmt.Errorf("cannot delete repository: %w", ErrDestructiveCallDisallowed)
	}

	_, err := r.c.Repositories.Delete(ctx, r.ref.GetIdentity(), r.ref.GetRepository())
	return handleHTTPError(err)
}

func newOrgRepository(ctx *clientContext, apiObj *github.Repository, ref gitprovider.RepositoryRef) *orgRepository {
	return &orgRepository{
		userRepository: *newUserRepository(ctx, apiObj, ref),
		teamAccess: &TeamAccessClient{
			clientContext: ctx,
			ref:           ref,
		},
	}
}

var _ gitprovider.OrgRepository = &orgRepository{}

type orgRepository struct {
	userRepository

	teamAccess *TeamAccessClient
}

func (r *orgRepository) TeamAccess() gitprovider.TeamAccessClient {
	return r.teamAccess
}

func validateRepositoryAPI(apiObj *github.Repository) error {
	validator := validation.New("GitHub.Repository")
	// Make sure name isn't nil
	if apiObj.Name == nil {
		validator.Required("Name")
	}
	if apiObj.Visibility != nil {
		v := gitprovider.RepositoryVisibility(*apiObj.Visibility)
		validator.Append(gitprovider.ValidateRepositoryVisibility(v), v, "Visibility")
	}
	// If there was a validation error, also mark it specifically as invalid server data
	if err := validator.Error(); err != nil {
		return validation.NewMultiError(err, gitprovider.ErrInvalidServerData)
	}
	return nil
}

func repositoryFromAPI(apiObj *github.Repository) gitprovider.RepositoryInfo {
	repo := gitprovider.RepositoryInfo{
		Description:   apiObj.Description,
		DefaultBranch: apiObj.DefaultBranch,
	}
	if apiObj.Visibility != nil {
		repo.Visibility = gitprovider.RepositoryVisibilityVar(gitprovider.RepositoryVisibility(*apiObj.Visibility))
	}
	return repo
}

func validateRepositoryAPIResp(apiObj *github.Repository, err error) (*github.Repository, error) {
	// If the response contained an error, return
	if err != nil {
		return nil, handleHTTPError(err)
	}
	// Make sure apiObj is valid
	if err := validateRepositoryAPI(apiObj); err != nil {
		return nil, err
	}
	return apiObj, nil
}

func repositoryToAPI(repo *gitprovider.RepositoryInfo, ref gitprovider.RepositoryRef) github.Repository {
	apiObj := github.Repository{
		Name: gitprovider.StringVar(ref.GetRepository()),
	}
	repositoryInfoToAPIObj(repo, &apiObj)
	return apiObj
}

func repositoryInfoToAPIObj(repo *gitprovider.RepositoryInfo, apiObj *github.Repository) {
	if repo.Description != nil {
		apiObj.Description = repo.Description
	}
	if repo.DefaultBranch != nil {
		apiObj.DefaultBranch = repo.DefaultBranch
	}
	if repo.Visibility != nil {
		apiObj.Visibility = gitprovider.StringVar(string(*repo.Visibility))
	}
}

func applyRepoCreateOptions(apiObj *github.Repository, opts gitprovider.RepositoryCreateOptions) {
	apiObj.AutoInit = opts.AutoInit
	if opts.LicenseTemplate != nil {
		apiObj.LicenseTemplate = gitprovider.StringVar(string(*opts.LicenseTemplate))
	}
}
