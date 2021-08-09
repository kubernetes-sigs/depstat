# Contributing Guidelines

Welcome to Kubernetes. We are excited about the prospect of you joining our [community](https://git.k8s.io/community)! The Kubernetes community abides by the CNCF [code of conduct](code-of-conduct.md). Here is an excerpt:

_As contributors and maintainers of this project, and in the interest of fostering an open and welcoming community, we pledge to respect all people who contribute through reporting issues, posting feature requests, updating documentation, submitting pull requests or patches, and other activities._

## Getting Started

We have full documentation on how to get started contributing here:

<!---
If your repo has certain guidelines for contribution, put them here ahead of the general k8s resources
-->

- [Contributor License Agreement](https://git.k8s.io/community/CLA.md) Kubernetes projects require that you sign a Contributor License Agreement (CLA) before we can accept your pull requests
- [Kubernetes Contributor Guide](https://git.k8s.io/community/contributors/guide) - Main contributor documentation, or you can just jump directly to the [contributing section](https://git.k8s.io/community/contributors/guide#contributing)
- [Contributor Cheat Sheet](https://git.k8s.io/community/contributors/guide/contributor-cheatsheet) - Common resources for existing developers

## Mentorship

- [Mentoring Initiatives](https://git.k8s.io/community/mentoring) - We have a diverse set of mentorship programs available that are always looking for volunteers!

<!---
Custom Information - if you're copying this template for the first time you can add custom content here, for example:

-->

## Contact Information

- [Slack channel](https://kubernetes.slack.com/messages/k8s-code-organization) 

## PR Workflow

1. Clone the repository by running 
```
https://github.com/kubernetes-sigs/depstat.git
```

2. Create another branch where you'll be making your changes by running,

```
git checkout -b branch_name
```

3. After making your changes you can test them by creating a binary of depstat with your changes. To do this run, `go build`. This will create a binary of depstat with your changes in the project root which you can use to test.

4. After you're satisfied with your changes, commit and push your branch:

```
git add .
git commit -s -m "commit message goes here"
git push origin branch_name
```

5. You can now use this branch to [create a pull request](https://docs.github.com/en/github/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request) to the `main` branch of the project.