/*
Copyright 2021 SPIRE Authors.

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

package stringset

import (
	"regexp"
)

type StringSet []string

func (ss StringSet) In(operand string) bool {
	for _, s := range ss {
		if s == operand {
			return true
		}
	}
	return false
}

func (ss StringSet) MatchRegex(operand string) bool {
	for _, s := range ss {
		match, _ := regexp.MatchString(s, operand)

		if match {
			return true
		}
	}
	return false
}
