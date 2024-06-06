# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXOPTS    ?=
SPHINXBUILD   ?= sphinx-build
SOURCEDIR     = source
BUILDDIR      = build

# Put it first so that "make" without argument is like "make help".
help:
	@$(SPHINXBUILD) -M help "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
	@echo -e "  doc8        doc8 linter"
	@echo -e "  lint        All documentation linters (including linkcheck)"
	@echo -e "  rstcheck    rstcheck linter"

.PHONY: doc8 help lint Makefile rstcheck server

doc8:
	doc8 --ignore D001 .

# Documentation linters
lint: doc8 linkcheck rstcheck

rstcheck:
	-rstcheck --recursive --warn-unknown-settings .

# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINXOPTS).
# Includes linkcheck
%: Makefile
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)


server:
	python3 -m http.server
