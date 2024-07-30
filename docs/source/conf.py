# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html
# -- Path setup --------------------------------------------------------------
# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
#
# import os
# import sys
# sys.path.insert(0, os.path.abspath('.'))
# -- Project information -----------------------------------------------------

project = "receptor"
copyright = "Red Hat Ansible"
author = "Red Hat Ansible"

# The full version, including alpha/beta/rc tags
# release = '0.0.0'


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = [
    "sphinx.ext.autosectionlabel",
    "pbr.sphinxext",
]

autosectionlabel_prefix_document = True

# Add any paths that contain templates here, relative to this directory.
templates_path = ["_templates"]

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ["Thumbs.db", ".DS_Store"]

pygments_style = "ansible"
language = "en"
master_doc = "index"
source_suffix = ".rst"

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_theme = "sphinx_ansible_theme"

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
# html_static_path = ['_static']

sidebar_collapse = False


# -- Options for HTML output -------------------------------------------------

# Output file base name for HTML help builder.
htmlhelp_basename = "receptorrdoc"

# -- Options for LaTeX output ---------------------------------------------

latex_elements = {
    # The paper size ('letterpaper' or 'a4paper').
    #
    # 'papersize': 'letterpaper',
    # The font size ('10pt', '11pt' or '12pt').
    #
    # 'pointsize': '10pt',
    # Additional stuff for the LaTeX preamble.
    #
    # 'preamble': '',
    # Latex figure (float) alignment
    #
    # 'figure_align': 'htbp',
}

latex_documents = [
    (master_doc, "receptor.tex", "receptor Documentation", "Red Hat Ansible", "manual"),
]

# -- Options for manual page output ---------------------------------------

# One entry per manual page. List of tuples
# (source start file, name, description, authors, manual section).
man_pages = [
    ("receptorctl/receptorctl_index", "receptorctl", "receptor client", [author], 1),
    (
        "receptorctl/receptorctl_connect",
        "receptorctl-connect",
        "Establishes a connection between local client and a Receptor node.",
        [author],
        1,
    ),
    ("receptorctl/receptorctl_ping", "receptorctl-ping", "Tests the network reachability of Receptor nodes.", [author], 1),
    ("receptorctl/receptorctl_reload", "receptorctl-reload", "Reloads the Receptor configuration for the connected node.", [author], 1),
    ("receptorctl/receptorctl_status", "receptorctl-status", "Displays the status of the Receptor network.", [author], 1),
    (
        "receptorctl/receptorctl_traceroute",
        "receptorctl-traceroute",
        "Displays the network route that packets follow to Receptor nodes.",
        [author],
        1,
    ),
    (
        "receptorctl/receptorctl_version",
        "receptorctl-version",
        "Displays version information for receptorctl and\
        the Receptor node to which it is connected.",
        [author],
        1,
    ),
    ("receptorctl/receptorctl_work_cancel", "receptorctl-work-cancel", "Terminates one or more units of work.", [author], 1),
    ("receptorctl/receptorctl_work_list", "receptorctl-work-list", "Displays known units of work.", [author], 1),
    ("receptorctl/receptorctl_work_release", "receptorctl-work-release", "Deletes one or more units of work.", [author], 1),
    ("receptorctl/receptorctl_work_results", "receptorctl-work-results", "Gets results for units of work.", [author], 1),
    ("receptorctl/receptorctl_work_submit", "receptorctl-work-submit", "Requests a Receptor node to run a unit of work.", [author], 1),
]

# -- Options for Texinfo output -------------------------------------------

# Grouping the document tree into Texinfo files. List of tuples
# (source start file, target name, title, author,
#  dir menu entry, description, category)
texinfo_documents = [
    (
        master_doc,
        "receptor",
        "receptor Documentation",
        author,
        "receptor",
        "Overlay network to establish a persistent mesh.",
        "Miscellaneous",
    ),
]

# -- Options for QtHelp output  -------------------------------------------


# -- Options for linkcheck builder  ---------------------------------------

linkcheck_report_timeouts_as_broken = False
linkcheck_timeout = 30

# -- Options for xml builder  ---------------------------------------------

xml_pretty = True

# -- Options for C domain  ------------------------------------------------


# -- Options for C++ domain  ----------------------------------------------


# -- Options for Python domain  -------------------------------------------


# -- Options for Javascript domain  ---------------------------------------
