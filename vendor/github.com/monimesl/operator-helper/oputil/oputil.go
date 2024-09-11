/*
 * Copyright 2021 - now, the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *       https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package oputil

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"k8s.io/apimachinery/pkg/util/yaml"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// Float64Or get the envVar as float64 otherwise the default
func Float64Or(envVar string, def float64) float64 {
	if val := Value(envVar); val != "" {
		return RequireFloat64(val)
	}
	return def
}

// Int32Or get the envVar as int32 otherwise the default
func Int32Or(envVar string, def int32) int32 {
	return int32(Int64Or(envVar, int64(def)))
}

// Int64Or get the envVar as int64 otherwise the default
func Int64Or(envVar string, def int64) int64 {
	if val := Value(envVar); val != "" {
		return int64(RequireFloat64(val))
	}
	return def
}

// RequireFloat64 returns the env variable as int64 or panic
func RequireFloat64(envVar string) float64 {
	i, err := strconv.ParseFloat(RequireValue(envVar), 64)
	if err != nil {
		panic(err)
	}
	return i
}

// Value returns the environment value with white space trimmed
func Value(envVar string) string {
	return strings.TrimSpace(os.Getenv(envVar))
}

// ValueOr returns the value of Value or default if none exists
func ValueOr(envVar, def string) string {
	if val := strings.TrimSpace(os.Getenv(envVar)); val != "" {
		return val
	}
	return def
}

// RequireValue returns the env Value or panic if none exists
func RequireValue(envVar string) string {
	if val := Value(envVar); val != "" {
		return val
	}
	log.Fatalf(fmt.Sprintf("expecting value for environment variable: %s", envVar))
	return ""
}

// RandomString generates a random base64 string of length len or err
func RandomString(size int) (string, error) {
	bitsNeeded := size * 6
	bytesNeeded := math.Ceil(float64(bitsNeeded) / 8)
	bs, err := RandomBytes(int(bytesNeeded))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bs)[:size], nil
}

// RandomBytes generates a random bytes of len len or error
func RandomBytes(ize int) ([]byte, error) {
	bs := make([]byte, ize)
	if _, err := rand.Read(bs); err != nil {
		return nil, err
	}
	return bs, nil
}

// Contains check if the haystack contains the key
func Contains(haystack []string, key string) bool {
	for _, item := range haystack {
		if key == item {
			return true
		}
	}
	return false
}

// ContainsWithPrefix check if the haystack contains a string with the prefix
func ContainsWithPrefix(haystack []string, prefix string) bool {
	for _, item := range haystack {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

// Remove returns a slice of haystack with the key removed if present
// otherwise returns the haystack itself
func Remove(key string, haystack []string) []string {
	for i, item := range haystack {
		if key == item {
			return append(haystack[:i], haystack[(i+1):]...)
		}
	}
	return haystack
}

// CreateConfigFromYamlString create a config string from a key-value map updated with the yaml extras excluding exclusions
func CreateConfigFromYamlString(extras string, name string, keyValues map[string]string, exclusions ...string) (string, map[string]string) {
	isIncluded := func(needle string) bool {
		for _, ex := range exclusions {
			if needle == ex {
				return false
			}
		}
		return true
	}
	if extras != "" {
		extrasMap := map[string]string{}
		if err := yaml.Unmarshal([]byte(extras), &extrasMap); err != nil {
			log.Printf("invalid %s data. reason: %s", name, err)
		}
		for k, v := range extrasMap {
			if !isIncluded(k) {
				log.Printf("The key: %s cannot be set directly to: '%s'. Skipping...", k, name)
				continue
			}
			keyValues[k] = v
		}
	}
	cfg := ""
	for k, v := range keyValues {
		if k == "" {
			log.Printf("Invalid key: %s for config: %s", k, name)
		} else if v != "" {
			// drop empty value cfg
			cfg += fmt.Sprintf("%s=%s\n", k, v)
			continue
		}
		delete(keyValues, k)
	}
	return cfg, keyValues
}

// IsOrdinalObjectIdle checks whether the ordinal object is idle.
// E.g for PVC, it means it's not longer attached to any pod.
func IsOrdinalObjectIdle(ordinalObjName string, replicas int) bool {
	index := strings.LastIndexAny(ordinalObjName, "-")
	if index > 0 {
		ordinal, err := strconv.Atoi(ordinalObjName[index+1:])
		if err != nil {
			return false
		}
		return ordinal >= replicas
	}
	return false
}
