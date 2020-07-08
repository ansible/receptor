#!/usr/bin/env python

from setuptools import setup, find_packages

with open('README.md', 'r') as f:
    long_description = f.read()

setup(
    name="receptor-python-worker",
    version="0.0.1",
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
