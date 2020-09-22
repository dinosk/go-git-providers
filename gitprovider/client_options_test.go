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
	"net/http"
	"reflect"
	"testing"

	"github.com/dinosk/go-git-providers/validation"
)

func roundTrippersEqual(a, b ChainableRoundTripperFunc) bool {
	if a == nil && b == nil {
		return true
	} else if (a != nil && b == nil) || (a == nil && b != nil) {
		return false
	}
	// Note that this comparison relies on "undefined behavior" in the Go language spec, see:
	// https://stackoverflow.com/questions/9643205/how-do-i-compare-two-functions-for-pointer-equality-in-the-latest-go-weekly
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

type commonClientOption interface {
	ApplyToCommonClientOptions(*CommonClientOptions) error
}

func makeOptions(opts ...commonClientOption) (*CommonClientOptions, error) {
	o := &CommonClientOptions{}
	for _, opt := range opts {
		if err := opt.ApplyToCommonClientOptions(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func withDomain(domain string) commonClientOption {
	return &CommonClientOptions{Domain: &domain}
}

func withDestructiveAPICalls(destructiveActions bool) commonClientOption {
	return &CommonClientOptions{EnableDestructiveAPICalls: &destructiveActions}
}

func withPreChainTransportHook(preRoundTripperFunc ChainableRoundTripperFunc) commonClientOption {
	return &CommonClientOptions{PreChainTransportHook: preRoundTripperFunc}
}

func withPostChainTransportHook(postRoundTripperFunc ChainableRoundTripperFunc) commonClientOption {
	return &CommonClientOptions{PostChainTransportHook: postRoundTripperFunc}
}

func dummyRoundTripper1(http.RoundTripper) http.RoundTripper { return nil }

func Test_makeOptions(t *testing.T) {
	tests := []struct {
		name         string
		opts         []commonClientOption
		want         *CommonClientOptions
		expectedErrs []error
	}{
		{
			name: "no options",
			want: &CommonClientOptions{},
		},
		{
			name: "withDomain",
			opts: []commonClientOption{withDomain("foo")},
			want: &CommonClientOptions{Domain: StringVar("foo")},
		},
		{
			name:         "withDomain, empty",
			opts:         []commonClientOption{withDomain("")},
			expectedErrs: []error{ErrInvalidClientOptions},
		},
		{
			name:         "withDomain, duplicate",
			opts:         []commonClientOption{withDomain("foo"), withDomain("bar")},
			expectedErrs: []error{ErrInvalidClientOptions},
		},
		{
			name: "withDestructiveAPICalls",
			opts: []commonClientOption{withDestructiveAPICalls(true)},
			want: &CommonClientOptions{EnableDestructiveAPICalls: BoolVar(true)},
		},
		{
			name:         "withDestructiveAPICalls, duplicate",
			opts:         []commonClientOption{withDestructiveAPICalls(true), withDestructiveAPICalls(false)},
			expectedErrs: []error{ErrInvalidClientOptions},
		},
		{
			name: "withPreChainTransportHook",
			opts: []commonClientOption{withPreChainTransportHook(dummyRoundTripper1)},
			want: &CommonClientOptions{PreChainTransportHook: dummyRoundTripper1},
		},
		{
			name:         "withPreChainTransportHook, duplicate",
			opts:         []commonClientOption{withPreChainTransportHook(dummyRoundTripper1), withPreChainTransportHook(dummyRoundTripper1)},
			expectedErrs: []error{ErrInvalidClientOptions},
		},
		{
			name: "withPostChainTransportHook",
			opts: []commonClientOption{withPostChainTransportHook(dummyRoundTripper1)},
			want: &CommonClientOptions{PostChainTransportHook: dummyRoundTripper1},
		},
		{
			name:         "withPostChainTransportHook, duplicate",
			opts:         []commonClientOption{withPostChainTransportHook(dummyRoundTripper1), withPostChainTransportHook(dummyRoundTripper1)},
			expectedErrs: []error{ErrInvalidClientOptions},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeOptions(tt.opts...)
			validation.TestExpectErrors(t, "makeOptions", err, tt.expectedErrs...)
			if tt.want == nil {
				return
			}
			if !roundTrippersEqual(got.PostChainTransportHook, tt.want.PostChainTransportHook) ||
				!roundTrippersEqual(got.PreChainTransportHook, tt.want.PreChainTransportHook) {
				t.Errorf("makeOptions() = %v, want %v", got, tt.want)
			}
			got.PostChainTransportHook = nil
			got.PreChainTransportHook = nil
			tt.want.PostChainTransportHook = nil
			tt.want.PreChainTransportHook = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
