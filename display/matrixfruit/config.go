// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrixfruit

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

type Clause struct {
	// is OTTL boolean expr
	Expression string `mapstructure:"expression"`

	// is a color name
	Color string `mapstructure:"color"`
}

type Config struct {
	// E.g., /dev/ttyACM0
	Device string `mapstructure:"device"`

	Metrics []string `mapstructure:"metrics"`

	Backgrounds []Clause `mapstructure:"backgrounds"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	return nil
}
