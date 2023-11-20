.. _contributing:

******************
Contributor guide
******************

Receptor is an open source project that lives at https://github.com/ansible/receptor

.. contents::
   :local:

Code of conduct
================

All project contributors must abide by the `Ansible Code of Conduct <https://docs.ansible.com/ansible/latest/community/code_of_conduct.html>`_.

Contributing
=============

Receptor welcomes community contributions!  See the :ref:`dev_guide` for details.

Release process
===============

Maintainers have the ability to run the `Stage Release <https://github.com/ansible/receptor/actions/workflows/stage.yml>`_ workflow. Running this workflow will:

- Build and push the container image to ghcr.io. This serves as a staging environment where the image can be tested.
- Create a draft release at `<https://github.com/ansible/receptor/releases>`_

After the draft release has been created, edit it and populate the description. Once you are done, click "Publish release".

After the release is published, the `Promote Release <https://github.com/ansible/receptor/actions/workflows/promote.yml>`_ workflow will run automatically. This workflow will:

- Publish receptorctl to `PyPI <https://pypi.org/project/receptorctl/>`_.
- Pull the container image from ghcr.io, re-tag, and push to `Quay.io <https://quay.io/repository/ansible/receptor>`_.
- Build binaries for various OSes/platforms, and attach them to the `release <https://github.com/ansible/receptor/releases>`_.

.. note::
  If you need to re-run `Stage Release <https://github.com/ansible/receptor/actions/workflows/stage.yml>`_ more than once you must delete the tag beforehand, otherwise the workflow will fail.
