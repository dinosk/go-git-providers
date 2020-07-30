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

package gitprovider

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/go-git-providers/validation"
)

// TODO: Add equality methods for IdentityRef and RepositoryRefs

// IdentityType is a typed string for what kind of identity type an IdentityRef is
type IdentityType string

const (
	// IdentityTypeUser represents an identity for a user account
	IdentityTypeUser = IdentityType("user")
	// IdentityTypeOrganization represents an identity for an organization
	IdentityTypeOrganization = IdentityType("organization")
	// IdentityTypeSuborganization represents an identity for a sub-organization
	IdentityTypeSuborganization = IdentityType("suborganization")
)

// IdentityRef references an organization or user account in a Git provider
type IdentityRef interface {
	// IdentityRef implements ValidateTarget so it can easily be validated as a field
	validation.ValidateTarget

	// GetDomain returns the URL-domain for the Git provider backend,
	// e.g. "github.com" or "self-hosted-gitlab.com:6443"
	GetDomain() string

	// GetIdentity returns the user account name or a slash-separated path of the
	// <organization-name>[/<sub-organization-name>...] form. This can be used as
	// an identifier for this specific actor in the system.
	GetIdentity() string

	// GetType returns what type of identity this instance represents. If IdentityTypeUser is returned
	// this IdentityRef can safely be casted to an UserRef. If any of IdentityTypeOrganization or
	// IdentityTypeSuborganization are returned, this IdentityRef can be casted to a OrganizationRef.
	GetType() IdentityType

	// RefIsEmpty returns true if all the parts of this IdentityRef are empty, otherwise false
	RefIsEmpty() bool

	// String returns the HTTPS URL, and implements fmt.Stringer
	String() string
}

// UserRef represents a reference to an user account in a Git provider. UserRef is a superset of
// IdentityRef.
type UserRef interface {
	IdentityRef

	// GetUserLogin returns the user account name
	GetUserLogin() string
}

// OrganizationRef represents a reference to a top-level- or sub-organization. OrganizationInfo
// is an implementation of this interface. OrganizationRef is a superset of IdentityRef.
type OrganizationRef interface {
	IdentityRef

	// GetOrganization returns the top-level organization, i.e. "fluxcd" or "kubernetes-sigs"
	GetOrganization() string
	// GetSubOrganizations returns the names of sub-organizations (or sub-groups),
	// e.g. ["engineering", "frontend"] would be returned for gitlab.com/fluxcd/engineering/frontend
	GetSubOrganizations() []string
}

// RepositoryRef references a repository hosted by a Git provider
type RepositoryRef interface {
	// RepositoryRef requires an IdentityRef to fully-qualify a repo reference
	IdentityRef

	// GetRepository returns the name of the repository. This name never includes the ".git"-suffix.
	GetRepository() string
}

// UserInfo implements UserRef. UserInfo represents an user account in a Git provider.
type UserInfo struct {
	// Domain returns e.g. "github.com", "gitlab.com" or a custom domain like "self-hosted-gitlab.com" (GitLab)
	// The domain _might_ contain port information, in the form of "host:port", if applicable
	// +required
	Domain string `json:"domain"`

	// UserLogin returns the user account login name.
	// +required
	UserLogin string `json:"userLogin"`
}

// UserInfo implements IdentityRef
var _ IdentityRef = UserInfo{}

// GetDomain returns the the domain part of the endpoint, can include port information.
func (u UserInfo) GetDomain() string {
	return u.Domain
}

// GetIdentity returns the identity of this actor, which in this case is the user login name
func (u UserInfo) GetIdentity() string {
	return u.GetUserLogin()
}

// GetType marks this UserInfo as being a IdentityTypeUser
func (u UserInfo) GetType() IdentityType {
	return IdentityTypeUser
}

// GetUserLogin returns the user login name
func (u UserInfo) GetUserLogin() string {
	return u.UserLogin
}

// String returns the HTTPS URL to access the User
func (u UserInfo) String() string {
	return fmt.Sprintf("https://%s/%s", u.GetDomain(), u.GetIdentity())
}

// RefIsEmpty returns true if all the parts of the given IdentityInfo are empty, otherwise false
func (u UserInfo) RefIsEmpty() bool {
	return len(u.Domain) == 0 && len(u.UserLogin) == 0
}

// ValidateFields validates its own fields for a given validator
func (u UserInfo) ValidateFields(validator validation.Validator) {
	// Require the Domain and Organization to be set
	if len(u.Domain) == 0 {
		validator.Required("Domain")
	}
	if len(u.UserLogin) == 0 {
		validator.Required("UserLogin")
	}
}

// OrganizationInfo implements IdentityRef
var _ IdentityRef = OrganizationInfo{}

// OrganizationInfo is an implementation of OrganizationRef
type OrganizationInfo struct {
	// Domain returns e.g. "github.com", "gitlab.com" or a custom domain like "self-hosted-gitlab.com" (GitLab)
	// The domain _might_ contain port information, in the form of "host:port", if applicable
	// +required
	Domain string `json:"domain"`

	// Organization specifies the URL-friendly, lowercase name of the organization or user account name,
	// e.g. "fluxcd" or "kubernetes-sigs".
	// +required
	Organization string `json:"organization"`

	// SubOrganizations point to optional sub-organizations (or sub-groups) of the given top-level organization
	// in the Organization field. E.g. "gitlab.com/fluxcd/engineering/frontend" would yield ["engineering", "frontend"]
	// +optional
	SubOrganizations []string `json:"subOrganizations"`
}

// GetDomain returns the the domain part of the endpoint, can include port information.
func (o OrganizationInfo) GetDomain() string {
	return o.Domain
}

// GetIdentity returns the identity of this actor, which in this case is the user login name
func (o OrganizationInfo) GetIdentity() string {
	orgParts := append([]string{o.GetOrganization()}, o.GetSubOrganizations()...)
	return strings.Join(orgParts, "/")
}

// GetType marks this UserInfo as being a IdentityTypeUser
func (o OrganizationInfo) GetType() IdentityType {
	if len(o.SubOrganizations) > 0 {
		return IdentityTypeSuborganization
	}
	return IdentityTypeOrganization
}

// GetOrganization returns top-level organization name
func (o OrganizationInfo) GetOrganization() string {
	return o.Organization
}

// GetOrganization returns sub-organization names
func (o OrganizationInfo) GetSubOrganizations() []string {
	return o.SubOrganizations
}

// String returns the HTTPS URL to access the Organization
func (o OrganizationInfo) String() string {
	return fmt.Sprintf("https://%s/%s", o.GetDomain(), o.GetIdentity())
}

// RefIsEmpty returns true if all the parts of the given IdentityInfo are empty, otherwise false
func (o OrganizationInfo) RefIsEmpty() bool {
	return len(o.Domain) == 0 && len(o.Organization) == 0 && len(o.SubOrganizations) == 0
}

// ValidateFields validates its own fields for a given validator
func (o OrganizationInfo) ValidateFields(validator validation.Validator) {
	// Require the Domain and Organization to be set
	if len(o.Domain) == 0 {
		validator.Required("Domain")
	}
	if len(o.Organization) == 0 {
		validator.Required("Organization")
	}
}

// RepositoryInfo is an implementation of RepositoryRef
type RepositoryInfo struct {
	// RepositoryInfo embeds an in IdentityRef inline
	IdentityRef `json:",inline"`

	// Name specifies the Git repository name. This field is URL-friendly,
	// e.g. "kubernetes" or "cluster-api-provider-aws"
	// +required
	RepositoryName string `json:"repositoryName"`
}

// RepositoryInfo implements the RepositoryRef interface
var _ RepositoryRef = RepositoryInfo{}

// GetRepository returns the name of the repository. This name never includes the ".git"-suffix.
func (r RepositoryInfo) GetRepository() string {
	return r.RepositoryName
}

// String returns the HTTPS URL to access the Repository
func (r RepositoryInfo) String() string {
	return fmt.Sprintf("%s/%s", r.IdentityRef.String(), r.GetRepository())
}

// RefIsEmpty returns true if all the parts of the given RepositoryInfo are empty, otherwise false
func (r RepositoryInfo) RefIsEmpty() bool {
	return (r.IdentityRef == nil || r.IdentityRef.RefIsEmpty()) && len(r.RepositoryName) == 0
}

// ValidateFields validates its own fields for a given validator
func (r RepositoryInfo) ValidateFields(validator validation.Validator) {
	// First, validate the embedded IdentityRef
	r.IdentityRef.ValidateFields(validator)
	// Require RepositoryName to be set
	if len(r.RepositoryName) == 0 {
		validator.Required("RepositoryName")
	}
}

// GetCloneURL gets the clone URL for the specified transport type
func (r RepositoryInfo) GetCloneURL(transport TransportType) string {
	return GetCloneURL(r, transport)
}

// GetCloneURL returns the URL to clone a repository for a given transport type. If the given
// TransportType isn't known an empty string is returned.
func GetCloneURL(rs RepositoryRef, transport TransportType) string {
	switch transport {
	case TransportTypeHTTPS:
		return fmt.Sprintf("%s.git", rs.String())
	case TransportTypeGit:
		return fmt.Sprintf("git@%s:%s/%s.git", rs.GetDomain(), rs.GetIdentity(), rs.GetRepository())
	case TransportTypeSSH:
		return fmt.Sprintf("ssh://git@%s/%s/%s", rs.GetDomain(), rs.GetIdentity(), rs.GetRepository())
	}
	return ""
}

// ParseOrganizationURL parses an URL to an organization into a OrganizationRef object
func ParseOrganizationURL(o string) (OrganizationRef, error) {
	// Always return IdentityInfo dereferenced, not as a pointer
	orgInfoPtr, err := parseOrganizationURL(o)
	if err != nil {
		return nil, err
	}
	return orgInfoPtrToOrganizationRef(orgInfoPtr)
}

// ParseUserURL parses an URL to an organization into a UserRef object
func ParseUserURL(u string) (UserRef, error) {
	// Use the same logic as for parsing organization URLs, but return an UserInfo object
	orgInfoPtr, err := parseOrganizationURL(u)
	if err != nil {
		return nil, err
	}
	userRef, err := orgInfoPtrToUserRef(orgInfoPtr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, u)
	}
	return userRef, nil
}

// ParseRepositoryURL parses a HTTPS or SSH clone URL into a RepositoryRef object
func ParseRepositoryURL(r string, isOrganization bool) (RepositoryRef, error) {
	// First, parse the URL as an organization
	orgInfoPtr, err := parseOrganizationURL(r)
	if err != nil {
		return nil, err
	}
	// The "repository" part of the URL parsed as an organization, is the last "sub-organization"
	// Check that there's at least one sub-organization
	if len(orgInfoPtr.SubOrganizations) < 1 {
		return nil, fmt.Errorf("%w: %s", ErrURLMissingRepoName, r)
	}

	// The repository name is the last "sub-org"
	repoName := orgInfoPtr.SubOrganizations[len(orgInfoPtr.SubOrganizations)-1]
	// Remove the repository name from the sub-org list
	orgInfoPtr.SubOrganizations = orgInfoPtr.SubOrganizations[:len(orgInfoPtr.SubOrganizations)-1]

	// Depending on the isOrganization flag, set the embedded identityRef to the right struct
	var identityRef IdentityRef
	if isOrganization {
		identityRef, err = orgInfoPtrToOrganizationRef(orgInfoPtr)
	} else {
		identityRef, err = orgInfoPtrToUserRef(orgInfoPtr)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrURLInvalid, r)
	}

	// Return the new RepositoryInfo
	return RepositoryInfo{
		// Never include any .git suffix at the end of the repository name
		RepositoryName: strings.TrimSuffix(repoName, ".git"),
		IdentityRef:    identityRef,
	}, nil
}

func parseURL(str string) (*url.URL, []string, error) {
	// Fail-fast if the URL is empty
	if len(str) == 0 {
		return nil, nil, fmt.Errorf("url cannot be empty: %w", ErrURLInvalid)
	}
	u, err := url.Parse(str)
	if err != nil {
		return nil, nil, err
	}
	// Only allow explicit https URLs
	if u.Scheme != "https" {
		return nil, nil, fmt.Errorf("%w: %s", ErrURLUnsupportedScheme, str)
	}
	// Don't allow any extra things in the URL, in order to be able to do a successful
	// round-trip of parsing the URL and encoding it back to a string
	if len(u.Fragment) != 0 || len(u.RawQuery) != 0 || len(u.User.String()) != 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrURLUnsupportedParts, str)
	}

	// Strip any leading and trailing slash to be able to split the string cleanly
	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	// Split the path by slash
	parts := strings.Split(path, "/")
	// Make sure there aren't any "empty" string splits
	// This has the consequence that it's guaranteed that there is at least one
	// part returned, so there's no need to check for len(parts) < 1
	for _, p := range parts {
		// Make sure any path part is not empty
		if len(p) == 0 {
			return nil, nil, fmt.Errorf("%w: %s", ErrURLInvalid, str)
		}
	}
	return u, parts, nil
}

// parseOrganizationURL parses the string into an OrganizationInfo object
func parseOrganizationURL(o string) (*OrganizationInfo, error) {
	u, parts, err := parseURL(o)
	if err != nil {
		return nil, err
	}
	// Create the IdentityInfo object
	info := &OrganizationInfo{
		Domain:           u.Host,
		Organization:     parts[0],
		SubOrganizations: []string{},
	}
	// If we've got more than one part, assume they are sub-organizations
	if len(parts) > 1 {
		info.SubOrganizations = parts[1:]
	}
	return info, nil
}

func orgInfoPtrToOrganizationRef(orgInfoPtr *OrganizationInfo) (OrganizationRef, error) {
	return *orgInfoPtr, nil
}

func orgInfoPtrToUserRef(orgInfoPtr *OrganizationInfo) (UserRef, error) {
	// Don't tolerate that there are "sub-parts" for an user URL
	if len(orgInfoPtr.GetSubOrganizations()) > 0 {
		return nil, ErrURLInvalid
	}
	// Return an UserInfo struct
	return UserInfo{
		Domain:    orgInfoPtr.Domain,
		UserLogin: orgInfoPtr.Organization,
	}, nil
}