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

func NewFromGroupsMap(gm GroupsMap) *Config {
	mapCopy := make(GroupsMap)

	for k, v := range gm {
		mapCopy[k] = v
	}

	return &Config{
		groups: mapCopy,
	}
}

func (c *Config) SetGroup(name string, group Group) {
	c.mu.Lock()
	if c.groups == nil {
		c.groups = make(GroupsMap)
	}

	c.groups[name] = group
	c.mu.Unlock()
}

func (c *Config) GetGroup(name string) (Group, bool) {
	c.mu.RLock()
	result, ok := c.groups[name]
	c.mu.RUnlock()

	return result, ok
}

func (c *Config) DeleteGroup(name string) {
	c.mu.Lock()
	delete(c.groups, name)
	c.mu.Unlock()
}

func (c *Config) GroupNames() []string {
	var keys []string

	c.mu.RLock()
	for key := range c.groups {
		keys = append(keys, key)
	}
	c.mu.RUnlock()

	return keys
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

	// handle group removals
	for _, name := range c.GroupNames() {
		if _, ok := config.Groups[name]; !ok {
			c.DeleteGroup(name)
		}
	}

	// always update internal groups when loading from file
	for gname, gconfig := range config.Groups {
		c.SetGroup(gname, gconfig)
	}

	return nil
}

// ValidUserGroups matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func (c *Config) ValidUserGroups(userGroups []string) GroupsMap {
	finalGroups := make(GroupsMap)

	for _, userGroup := range userGroups {
		if v, ok := c.GetGroup(userGroup); ok {
			finalGroups[userGroup] = v
		}
	}

	return finalGroups
}

type ConfigWatcher struct {
	ErrorChan chan error
	StopChan  chan bool
}

func (c *Config) Watch(filePath string) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(filePath); err != nil {
		return nil, err
	}

	if err := c.Load(filePath); err != nil {
		return nil, fmt.Errorf("error loading config: %v", err)
	}

	stopChan := make(chan bool)

	configWatcher := ConfigWatcher{
		ErrorChan: watcher.Errors,
		StopChan:  stopChan,
	}

	go c.watch(filePath, watcher, configWatcher.StopChan)
	return &configWatcher, nil
}

func (c *Config) watch(filePath string, watcher *fsnotify.Watcher, stopChan chan bool) {
	defer watcher.Close()
	for {
		select {
		case event := <-watcher.Events:
			var reload bool
			// Mounted files are symlinks. When the kubelet refreshes the file it is removing
			// and adding a symlink.  Therefore, when we see a remove event we know that a reload
			// needs to take place.
			// https://kubernetes.io/docs/concepts/configuration/secret/#secret-files-permissions
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				if err := watcher.Remove(event.Name); err != nil {
					log.Errorf("error removing watcher after file has been removed: %v", err)
				}
				if err := watcher.Add(event.Name); err != nil {
					log.Errorf("error re-watching config after file has been removed: %v", err)
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
		case <-stopChan:
			return
		}
	}
}
