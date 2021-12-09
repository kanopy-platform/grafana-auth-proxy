package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

type (
	Config struct {
		mu     sync.RWMutex
		groups GroupsMap
	}

	// for marshalling purposes
	config struct {
		Groups GroupsMap `json:"groups"`
	}

	GroupsMap map[string]Group

	Group struct {
		GrafanaAdmin bool  `json:"grafanaAdmin,omitempty"`
		Orgs         []Org `json:"orgs"`
	}

	Org struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	}
)

func New() *Config {
	groupsMap := make(GroupsMap)

	return &Config{
		groups: groupsMap,
	}
}

func (c *Config) SetGroup(name string, group Group) {
	c.mu.Lock()
	c.groups[name] = group
	c.mu.Unlock()
}

func (c *Config) LoadGroup(name string) (Group, bool) {
	c.mu.RLock()
	result, ok := c.groups[name]
	c.mu.RUnlock()

	return result, ok
}

func (c *Config) GroupsMap() GroupsMap {
	return c.groups
}

func (c *Config) Load(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	config := &config{}
	if err = yaml.Unmarshal(data, config); err != nil {
		return err
	}

	// Always update internal groups when loading from file
	for gname, gconfig := range config.Groups {
		c.SetGroup(gname, gconfig)
	}

	return nil
}

// ValidUserGroups matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func ValidUserGroups(userGroups []string, groupsMap GroupsMap) GroupsMap {
	finalGroups := make(GroupsMap)

	for _, userGroup := range userGroups {
		if v, ok := groupsMap[userGroup]; ok {
			finalGroups[userGroup] = v
		}
	}

	return finalGroups
}

func (c *Config) Watch(filePath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(filePath); err != nil {
		return err
	}

	if err := c.Load(filePath); err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	go c.watch(filePath, watcher)
	return nil
}

func (c *Config) watch(filePath string, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	for {
		select {
		case event := <-watcher.Events:
			reload := false
			// Mounted files are symlinks. When the kubelet refreshes the file it is removing
			// and adding a symlink.  Therefore, when we see a remove event we know that a reload
			// needs to take place.
			// https://kubernetes.io/docs/concepts/configuration/secret/#secret-files-permissions

			if event.Op&fsnotify.Remove == fsnotify.Remove {
				watcher.Remove(event.Name)
				if err := watcher.Add(event.Name); err != nil {
					log.Errorf("error re-watching config: %v", err)
				}
				reload = true
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				reload = true
			}

			if reload {
				if err := c.Load(event.Name); err != nil {
					log.Errorf("error refreshing config: %v", err)
				} else {
					log.Info("config file reloaded")
				}
			}
		case err, ok := <-watcher.Errors:
			log.Errorf("error on watcher reload: %v", err)
			if !ok {
				return
			}
		}
	}
}
