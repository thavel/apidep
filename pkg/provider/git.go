package provider

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	giturls "github.com/chainguard-dev/git-urls"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/go-git/go-git/v6/storage/memory"

	"github.com/thavel/apidep/pkg/file"
)

type Git struct{}

func (*Git) Match(uri string) bool {
	_, err := giturls.Parse(uri)
	return err == nil
}

func (*Git) Parse(uri, version string) (file.Source, error) {
	repo, err := openRepo(uri, version)
	if err != nil {
		return nil, err
	}
	h, err := resolveVersion(repo, version)
	if err != nil {
		return nil, fmt.Errorf("resolve version %q: %w", version, err)
	}
	return &gitSource{repo: repo, hash: h}, nil
}

type gitSource struct {
	repo *gogit.Repository
	hash plumbing.Hash
}

func (s *gitSource) Fetch(filePath string) ([]byte, error) {
	commit, err := s.repo.CommitObject(s.hash)
	if err != nil {
		return nil, fmt.Errorf("commit object: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}
	file, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("file %q not found in tree: %w", filePath, err)
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("file reader: %w", err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return content, nil
}

func (s *gitSource) Commit() string {
	return s.hash.String()
}

func openRepo(uri, version string) (*gogit.Repository, error) {
	clientOpts, err := buildAuth(uri)
	if err != nil {
		return nil, fmt.Errorf("git auth: %w", err)
	}

	cloneOpts := &gogit.CloneOptions{
		URL:           uri,
		ClientOptions: clientOpts,
		Depth:         1,
		NoCheckout:    true,
	}

	if version == "" {
		cloneOpts.SingleBranch = true
		repo, err := gogit.Clone(memory.NewStorage(), nil, cloneOpts)
		if err != nil {
			return nil, fmt.Errorf("clone: %w", err)
		}
		return repo, nil
	}

	// try as a branch.
	cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(version)
	cloneOpts.SingleBranch = true
	if repo, err := gogit.Clone(memory.NewStorage(), nil, cloneOpts); err == nil {
		return repo, nil
	}

	// try as a tag.
	cloneOpts.ReferenceName = plumbing.NewTagReferenceName(version)
	cloneOpts.Tags = plumbing.NoTags
	if repo, err := gogit.Clone(memory.NewStorage(), nil, cloneOpts); err == nil {
		return repo, nil
	}

	// full clone
	cloneOpts.ReferenceName = ""
	cloneOpts.SingleBranch = false
	cloneOpts.Depth = 0
	repo, err := gogit.Clone(memory.NewStorage(), nil, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	return repo, nil
}

func resolveVersion(repo *gogit.Repository, version string) (plumbing.Hash, error) {
	if version == "" {
		ref, err := repo.Head()
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("resolve HEAD: %w", err)
		}
		return ref.Hash(), nil
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(version), true)
	if err == nil {
		return ref.Hash(), nil
	}
	ref, err = repo.Reference(plumbing.NewTagReferenceName(version), true)
	if err == nil {
		obj, err := repo.TagObject(ref.Hash())
		if err == nil {
			return obj.Target, nil
		}
		return ref.Hash(), nil
	}
	h := plumbing.NewHash(version)
	if !h.IsZero() {
		return h, nil
	}
	return plumbing.ZeroHash, fmt.Errorf("cannot resolve %q as branch, tag or commit", version)
}

func buildAuth(source string) ([]client.Option, error) {
	if isSSHURI(source) {
		user := sshUser(source)
		auth, err := ssh.NewSSHAgentAuth(user)
		if err == nil {
			return []client.Option{client.WithSSHAuth(auth)}, nil
		}
		keyAuth, err := sshKeyFileAuth(user)
		if err != nil {
			return nil, fmt.Errorf("ssh auth: no agent available and no usable key file: %w", err)
		}
		return []client.Option{client.WithSSHAuth(keyAuth)}, nil
	}
	u, err := url.Parse(source)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("source %q is not a valid git clone uri", source)
	}
	if u.User != nil {
		pass, _ := u.User.Password()
		return []client.Option{client.WithHTTPAuth(&http.BasicAuth{Username: u.User.Username(), Password: pass})}, nil
	}
	return nil, nil
}

func isSSHURI(source string) bool {
	if strings.HasPrefix(source, "ssh://") {
		return true
	}
	if !strings.Contains(source, "://") && strings.Contains(source, "@") && strings.Contains(source, ":") {
		return true
	}
	return false
}

func sshUser(source string) string {
	if strings.HasPrefix(source, "ssh://") {
		u, err := url.Parse(source)
		if err == nil && u.User != nil {
			return u.User.Username()
		}
		return "git"
	}
	at := strings.Index(source, "@")
	if at >= 0 {
		return source[:at]
	}
	return "git"
}

func sshKeyFileAuth(user string) (*ssh.PublicKeys, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	candidates := []string{"id_ed25519", "id_ecdsa", "id_rsa"}
	sshDir := home + "/.ssh"
	for _, name := range candidates {
		path := sshDir + "/" + name
		if _, err := os.Stat(path); err != nil {
			continue
		}
		auth, err := ssh.NewPublicKeysFromFile(user, path, "")
		if err == nil {
			return auth, nil
		}
	}
	return nil, fmt.Errorf("no readable private key found in %s", sshDir)
}
