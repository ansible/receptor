#!/usr/bin/env python

from setuptools import setup, find_packages
import os
import json

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
    verinfo = json.load(f)

setup(
    name="receptorctl",
    version=verinfo['version'],
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
