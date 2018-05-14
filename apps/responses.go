package apps

type File struct {
	Path     string `json:"path"`
	Hash     string `json:"hash,omitempty"`
	Location string `json:"location,omitempty"`
}

type FileList struct {
	Files []*File `json:"data"`
}

// DependencyTree is the recursive representation of dependencies
//      {
//          "foo.bar@1.2.3": {
//              "boo.zaz@2.0.1-beta": {},
//              "boo.yay@1.0.0": {}
//          },
//          "lala.ha@0.1.0": {}
//      }
type DependencyTree map[string]DependencyTree

// ActiveApp represents an active app's metadata
type ActiveApp struct {
	ID                   string            `json:"id"`
	Vendor               string            `json:"vendor"`
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Title                string            `json:"title"`
	Description          string            `json:"description"`
	Categories           []string          `json:"categories"`
	Dependencies         map[string]string `json:"dependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	SettingsSchema       interface{}       `json:"settingsSchema"`
	CredentialType       string            `json:"credentialType"`
	Policies             interface{}       `json:"policies"`
	Link                 string            `json:"link,omitempty"`
	IsRoot               *bool             `json:"_isRoot,omitempty"`
	DependencyTree       DependencyTree    `json:"_dependencyTree"`
	DependencySet        []string          `json:"_dependencySet"`
	ActivationDate       string            `json:"_activationDate"`
	Registry             string            `json:"_registry,omitempty"`
	ResolvedDependencies map[string]string `json:"_resolvedDependencies"`
}

// PublishedApp represents a published app's metadata
type PublishedApp struct {
	ID               string            `json:"id"`
	Vendor           string            `json:"vendor"`
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Categories       []string          `json:"categories"`
	Dependencies     map[string]string `json:"dependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
	SettingsSchema   interface{}       `json:"settingsSchema"`
	CredentialType   string            `json:"credentialType"`
	Policies         interface{}       `json:"policies"`
	Publisher        string            `json:"_publisher"`
	PublicationDate  string            `json:"_publicationDate"`
}

type RootApp struct {
	Apps     string `json:"app"`
	ID       string `json:"id"`
	Location string `json:"location"`
}

type RootAppList struct {
	Apps []*RootApp `json:"data"`
}

type AppList struct {
	Apps []*ActiveApp `json:"apps"`
}
