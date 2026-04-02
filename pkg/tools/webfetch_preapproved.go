package tools

import "strings"

// Source: tools/WebFetchTool/preapproved.ts

// preapprovedHostsOnly contains hostname-only preapproved entries.
// Source: preapproved.ts:14-131
var preapprovedHostsOnly = map[string]bool{
	// Anthropic
	"platform.claude.com": true,
	"code.claude.com":     true,
	"modelcontextprotocol.io": true,
	"agentskills.io": true,

	// Top Programming Languages
	"docs.python.org":       true,
	"en.cppreference.com":   true,
	"docs.oracle.com":       true,
	"learn.microsoft.com":   true,
	"developer.mozilla.org": true,
	"go.dev":                true,
	"pkg.go.dev":            true,
	"www.php.net":           true,
	"docs.swift.org":        true,
	"kotlinlang.org":        true,
	"ruby-doc.org":          true,
	"doc.rust-lang.org":     true,
	"www.typescriptlang.org": true,

	// Web & JavaScript Frameworks
	"react.dev":        true,
	"angular.io":       true,
	"vuejs.org":        true,
	"nextjs.org":       true,
	"expressjs.com":    true,
	"nodejs.org":       true,
	"bun.sh":           true,
	"jquery.com":       true,
	"getbootstrap.com": true,
	"tailwindcss.com":  true,
	"d3js.org":         true,
	"threejs.org":      true,
	"redux.js.org":     true,
	"webpack.js.org":   true,
	"jestjs.io":        true,
	"reactrouter.com":  true,

	// Python Frameworks
	"docs.djangoproject.com":   true,
	"flask.palletsprojects.com": true,
	"fastapi.tiangolo.com":    true,
	"pandas.pydata.org":       true,
	"numpy.org":               true,
	"www.tensorflow.org":      true,
	"pytorch.org":             true,
	"scikit-learn.org":        true,
	"matplotlib.org":          true,
	"requests.readthedocs.io": true,
	"jupyter.org":             true,

	// PHP Frameworks
	"laravel.com":   true,
	"symfony.com":   true,
	"wordpress.org": true,

	// Java Frameworks
	"docs.spring.io":     true,
	"hibernate.org":      true,
	"tomcat.apache.org":  true,
	"gradle.org":         true,
	"maven.apache.org":   true,

	// .NET & C#
	"asp.net":                 true,
	"dotnet.microsoft.com":    true,
	"nuget.org":               true,
	"blazor.net":              true,

	// Mobile Development
	"reactnative.dev":       true,
	"docs.flutter.dev":      true,
	"developer.apple.com":   true,
	"developer.android.com": true,

	// Data Science & ML
	"keras.io":          true,
	"spark.apache.org":  true,
	"huggingface.co":    true,
	"www.kaggle.com":    true,

	// Databases
	"www.mongodb.com":    true,
	"redis.io":           true,
	"www.postgresql.org": true,
	"dev.mysql.com":      true,
	"www.sqlite.org":     true,
	"graphql.org":        true,
	"prisma.io":          true,

	// Cloud & DevOps
	"docs.aws.amazon.com":   true,
	"cloud.google.com":      true,
	"kubernetes.io":         true,
	"www.docker.com":        true,
	"www.terraform.io":      true,
	"www.ansible.com":       true,
	"docs.netlify.com":      true,
	"devcenter.heroku.com":  true,
	"cypress.io":            true,
	"selenium.dev":          true,

	// Game Development
	"docs.unity.com":         true,
	"docs.unrealengine.com":  true,

	// Other Essential Tools
	"git-scm.com":   true,
	"nginx.org":     true,
	"httpd.apache.org": true,
}

// preapprovedPathPrefixes contains path-scoped preapproved entries.
// Source: preapproved.ts:136-152
var preapprovedPathPrefixes = map[string][]string{
	"github.com": {"/anthropics"},
	"vercel.com": {"/docs"},
}

// IsPreapprovedHost checks if a hostname+pathname is preapproved.
// Source: preapproved.ts:154-166
func IsPreapprovedHost(hostname, pathname string) bool {
	if preapprovedHostsOnly[hostname] {
		return true
	}
	prefixes, ok := preapprovedPathPrefixes[hostname]
	if ok {
		for _, p := range prefixes {
			// Enforce path segment boundaries
			if pathname == p || strings.HasPrefix(pathname, p+"/") {
				return true
			}
		}
	}
	return false
}

// IsPreapprovedURL checks if a URL string is preapproved.
// Source: utils.ts:130-137
func IsPreapprovedURL(rawURL string) bool {
	hostname, pathname := parseHostAndPath(rawURL)
	if hostname == "" {
		return false
	}
	return IsPreapprovedHost(hostname, pathname)
}
