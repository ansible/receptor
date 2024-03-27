******************
Contributor guide
******************

Receptor is an open source project that lives at https://github.com/ansible/receptor

.. contents::
   :local:

===============
Code of conduct
===============

All project contributors must abide by the `Ansible Code of Conduct <https://docs.ansible.com/ansible/latest/community/code_of_conduct.html>`_.

============
Contributing
============

Receptor welcomes community contributions!  See the :ref:`dev_guide` for information about receptor development.

-------------
Pull requests
-------------
Contributions to Receptor go through the Github pull request process.

An initial checklist for your change to increase the likelihood of acceptance:
- No issues when running linters/code checkers
- No issues from unit/functional tests
- Write desriptive and meaningful commit messages. See `How to write a Git commit message <https://cbea.ms/git-commit/>`_ and `Learn to write good commit message and description <https://gist.github.com/webknjaz/cb7d7bf62c3dda4b1342d639d0e78d79>`_.

===============
Release process
===============

Maintainers have the ability to run the `Stage Release`_ workflow. Running this workflow will:

- Build and push the container image to ghcr.io. This serves as a staging environment where the image can be tested.
- Create a draft release at `<https://github.com/ansible/receptor/releases>`_

After the draft release has been created, edit it and populate the description. Once you are done, click "Publish release".

After the release is published, the `Promote Release <https://github.com/ansible/receptor/actions/workflows/promote.yml>`_ workflow will run automatically. This workflow will:

- Publish receptorctl to `PyPI <https://pypi.org/project/receptorctl/>`_.
- Pull the container image from ghcr.io, re-tag, and push to `Quay.io <https://quay.io/repository/ansible/receptor>`_.
- Build binaries for various OSes/platforms, and attach them to the `release <https://github.com/ansible/receptor/releases>`_.

.. note::
  If you need to re-run `Stage Release`_ more than once you must delete the tag beforehand, otherwise the workflow will fail.

.. _Stage Release: https://github.com/ansible/receptor/actions/workflows/stage.yml

