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

package spireapi

var (
	// TODO: optimize batch/page sizes
	// These batch sizes are vars so they can be adjusted during tests.

	entryCreateBatchSize = 50
	entryUpdateBatchSize = 50
	entryDeleteBatchSize = 200
	entryListPageSize    = 200

	federationRelationshipCreateBatchSize = 50
	federationRelationshipUpdateBatchSize = 50
	federationRelationshipDeleteBatchSize = 200
	federationRelationshipListPageSize    = 200
)

func runBatch(size, batch int, fn func(start, end int) error) error {
	if batch < 1 {
		batch = size
	}
	for i := 0; i < size; {
		n := size - i
		if n > batch {
			n = batch
		}
		err := fn(i, i+n)
		if err != nil {
			return err
		}
		i += n
	}
	return nil
}
