Release process
===============

Maintainers have the ability to run the `Stage Release <https://github.com/ansible/receptor/actions/workflows/stage.yml>`_ workflow. Running this workflow will:

- Build and push the container image to ghcr.io. This serves as a staging environment where the image can be tested.
- Create a draft release at `<https://github.com/ansible/receptor/releases>`_

After the draft release has been created, edit it and populate the description. Once you are done, click "Publish release".

After the release is published, the `Promote Release <https://github.com/ansible/receptor/actions/workflows/promote.yml>`_ workflow will run automatically. This workflow will:

- Publish receptorctl to PyPi.
- Pull the container image from ghcr.io, re-tag, and push to quay.io.
