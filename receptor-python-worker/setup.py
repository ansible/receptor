#!/usr/bin/env python

import os
from setuptools import setup, find_packages

with open('README.md', 'r') as f:
    long_description = f.read()

verfile = None
for fn in ['VERSION', '../VERSION']:
    if os.path.exists(fn):
        verfile = fn
        break
if verfile is None:
    raise IOError("Version file not found.")
with open(verfile, 'r') as f:
    version = f.readline().rstrip('\n\r')

setup(
    name="receptor-python-worker",
    version=version,
    author='Red Hat Ansible',
    url="https://github.com/project-receptor/receptor/receptor-python-worker",
    license='Apache',
    packages=find_packages(),
    long_description=long_description,
    long_description_content_type='text/markdown',
    install_requires=[
        "setuptools",
    ],
    zip_safe=False,
    entry_points={
        'console_scripts': [
              'receptor-python-worker=receptor_python_worker:run'
          ]
    },
    classifiers=[
        "Programming Language :: Python :: 3",
    ],
)
