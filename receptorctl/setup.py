#!/usr/bin/env python

import os
import subprocess
from setuptools import setup, find_packages

with open('README.md', 'r') as f:
    long_description = f.read()

verfile = None
for fn in ['.VERSION', '../.VERSION']:
    if os.path.exists(fn):
        verfile = fn
        break

if verfile is None:
    subprocess.run(["make", "version"], cwd="../")
    verfile = '../.VERSION'

    if not os.path.exists(verfile):
        raise IOError("Version file not found.")

with open(verfile, 'r') as f:
    version = f.readline().rstrip('\n\r')

setup(
    name="receptorctl",
    version=version,
    author='Red Hat',
    url="https://github.com/project-receptor/receptor/receptorctl",
    license='APL 2.0',
    packages=find_packages(),
    long_description=long_description,
    long_description_content_type='text/markdown',
    install_requires=[
        "setuptools",
        "python-dateutil",
        "click",
    ],
    zip_safe=False,
    entry_points={
        'console_scripts': [
              'receptorctl=receptorctl:run'
          ]
    },
    classifiers=[
        "Programming Language :: Python :: 3",
    ],
)
