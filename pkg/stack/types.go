// Copyright Nitric Pty Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stack

const (
	Aws          = "aws"
	Azure        = "azure"
	Gcp          = "gcp"
	Digitalocean = "digitalocean"
)

var Providers = []string{Aws, Azure, Gcp, Digitalocean}

type Config struct {
	Name     string                 `json:"name,omitempty"`
	Provider string                 `json:"provider,omitempty"`
	Region   string                 `json:"region,omitempty"`
	Extra    map[string]interface{} `json:",inline,omitempty"`
}