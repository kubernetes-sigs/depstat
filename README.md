# depstat

`depstat` is a command-line tool for analyzing dependencies of Go modules enabled projects. 

![depstat demo with k8s repo](./depstat-k8s-demo.gif)

## Installation 
To install depstat you can run

```
go get github.com/kubernetes-sigs/depstat@latest
```

## Usage
`depstat` can be used as a standalone command-line application. You can navigate to your go modules enabled project and use `depstat` to produce metrics about your project. Another common way to run `depstat` is in the CI pipeline of your project. This would help you analyze the dependency changes which come with PRs. You can look at how this is done for the [kubernetes/kuberenets](https://github.com/kubernetes/kubernetes) repo using [prow](https://github.com/kubernetes/test-infra/tree/master/prow) [here](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-arch/kubernetes-depstat.yaml). 

## Commands

To see the list of commands `depstat` offers you can run `depstat help`. `depstat` currently supports the following commands:

### cycles

`depstat cycles` shows all the cycles present in the dependencies of the project.

An example of a cycle in project dependenies is:
`golang.org/x/net -> golang.org/x/crypto -> golang.org/x/net`

`--json` prints the output of the cycles command in JSON format. For the above example the JSON output would look like this:
```
{
  "cycles": [
    [
      "golang.org/x/net",
      "golang.org/x/crypto",
      "golang.org/x/net"
    ]
  ]
}
```

### graph

`depstat graph` will generate a `graph.dot` file which can be used with [Graphviz](https://graphviz.org)'s dot command to visualize the dependencies of a project.

For example, after running `depstat graph`, an SVG can be created using:
`twopi -Tsvg -o dag.svg graph.dot`

By default, the graph would be created around the main module (first module in the `go mod graph` output), but you can choose to create a graph around a particular dependency using the `--dep` flag.

### list

`depstat list` shows a sorted list of all project dependencies. These include both direct and transitive dependencies.

1. Direct dependencies: Dependencies that are directly used in the code of the project. These do not include standard go packages like `fmt`, etc. These are dependencies that appear on the right side of the main module in the `go mod graph` output.

2. Transitive dependencies: These are dependencies that get imported because they are needed by some direct dependency of the project. These are dependencies that appear on the right side of a dependency that isn't the main module in the `go mod graph` output.

### stats

`depstat stats` will provide the following metrics about the dependencies of the project:

1. Direct Dependencies: Total number of dependencies required by the [main module(s)](#main-module) directly.

2. Transitive Dependencies: Total number of transitive dependencies (dependencies which are further needed by direct dependencies of the project).

3. Total Dependencies: Total number of dependencies of the [main module(s)](#main-modules(s)).

4. Max Depth of Dependencies: Length of the longest chain starting from the first [main module](#main-modules(s)); defaults to length from the first module encountered in "go mod graph" output.

- The `--json` flag gives this output in a JSON format.
- `--verbose` mode will help provide you with the list of all the dependencies and will also print the longest dependency chain.

#### main module
By default, the first module encountered in "go mod graph" output is treated as the main module by `depstat`. Depstat uses this main module to determine the direct and transitive dependencies. This behavior can be changed by specifying the main module manually using the `--mainModules` flag with the stats command. The flag takes a list of modules names, for example:

```
depstat stats -m="k8s.io/kubernetes,k8s.io/kubectl"
```

## Project Goals
`depstat` is being developed under the code organization sub-project under [SIG Architecture](https://github.com/kubernetes/community/tree/master/sig-architecture). The goal is to make it easy to evaluate dependency updates to Kubernetes. This is done by running `depstat` as part of the Kubernetes CI pipeline.

## Community Contact Information
You can reach the maintainers of this project at:

[#k8s-code-organization](https://kubernetes.slack.com/messages/k8s-code-organization) on the [Kubernetes slack](http://slack.k8s.io).

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

