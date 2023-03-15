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
  To customize packaging and allow for tags that do not strictly satisfy the semantic versioning given to development releases, the current workflow uses simple ``make`` to build and archive binaries, instead of specific tools for build and release.
