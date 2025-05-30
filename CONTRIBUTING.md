# Receptor

Hi there! We're excited to have you as a contributor.

Have questions about this document or anything not covered here? Create a topic using the [AAP tag on the Ansible Forum](https://forum.ansible.com/tag/aap).

## Table of contents

- [Things to know prior to submitting code](#things-to-know-prior-to-submitting-code)
- [Setting up your development environment](#setting-up-your-development-environment)
  - [Fork and clone the Receptor repo](#fork-and-clone-the-receptor-repo)
  - [Development Requirements](#development-requirements)
  - [Build and Run the Development Environment](#build-and-run-the-development-environment)
- [What should I work on?](#what-should-i-work-on)
- [Submitting Pull Requests](#submitting-pull-requests)
- [Reporting Issues](#reporting-issues)
- [Getting Help](#getting-help)

## Things to know prior to submitting code

- All code submissions are done through pull requests against the `devel` branch.
- You must use `git commit --signoff` for any commit to be merged, and agree that usage of --signoff constitutes agreement with the terms of [DCO 1.1](./DCO_1_1.md).
- Take care to make sure no merge commits are in the submission, and use `git rebase` vs `git merge` for this reason.
  - If collaborating with someone else on the same branch, consider using `--force-with-lease` instead of `--force`. This will prevent you from accidentally overwriting commits pushed by someone else. For more information, see [git push docs](https://git-scm.com/docs/git-push#git-push---force-with-leaseltrefnamegt).
- If submitting a large code change, it's a good idea to create a [forum topic tagged with 'aap'](https://forum.ansible.com/tag/aap), and talk about what you would like to do or add first. This not only helps everyone know what's going on, it also helps save time and effort, if the community decides some changes are needed.
- We ask all of our community members and contributors to adhere to the [Ansible code of conduct](http://docs.ansible.com/ansible/latest/community/code_of_conduct.html). If you have questions, or need assistance, please reach out to our community team at [codeofconduct@ansible.com](mailto:codeofconduct@ansible.com)

## Setting up your development environment

Our team uses [VS Code](https://code.visualstudio.com/) with the [Golang extension](https://marketplace.visualstudio.com/items?itemName=golang.Go) installed for our development environments. The instrustions below will show how to set up an environment using this tool set.

### Fork and clone the Receptor repo

If you have not done so already, you'll need to fork the Receptor repo on GitHub. For more on how to do this, see [Fork a Repo](https://help.github.com/articles/fork-a-repo/).

### Development Requirements

- [Git](https://git-scm.com/book/en/v2)
- [Golang](https://go.dev/doc/install)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [make](https://www.gnu.org/software/make/manual/make.html)

### Build and Run the Development Environment

#### Building Receptor
`make build-all`

#### Running tests
`make test`

## What should I work on?

We have a ["good first issue" label](https://github.com/ansible/receptor/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) we put on some issues that might be a good starting point for new contributors.

Fixing bugs and updating the documentation are always appreciated, so reviewing the backlog of issues is always a good place to start.

## Submitting Pull Requests

Fixes and Features for Receptor will go through the Github pull request process. Submit your pull request (PR) against the `devel` branch.

Here are a few things you can do to help the visibility of your change, and increase the likelihood that it will be accepted:

- No issues when running linters/code checkers
- No issues from unit tests
- Write tests for new functionality, update/add tests for bug fixes
- Make the smallest change possible
- Write good commit messages. See [How to write a Git commit message](https://chris.beams.io/posts/git-commit/).


We like to keep our commit history clean, and will require resubmission of pull requests that contain merge commits. Use `git pull --rebase`, rather than
`git pull`, and `git rebase`, rather than `git merge`.

Sometimes it might take us a while to fully review your PR. We try to keep the `devel` branch in good working order, and so we review requests carefully. Please be patient.

When your PR is initially submitted the checks will not be run until a maintainer allows them to be. Once a maintainer has done a quick review of your work the PR will have the linter and unit tests run against them via GitHub Actions, and the status reported in the PR.

## Reporting Issues

We welcome your feedback, and encourage you to file an issue when you run into a problem.

## Getting Help

If you require additional assistance, please submit your question to the [Ansible Forum](https://forum.ansible.com/tag/aap).
