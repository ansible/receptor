.. receptor documentation master file, created by
   sphinx-quickstart on Sat May  1 19:39:15 2021.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

###############################
Ansible Receptor documentation
###############################

Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers. Receptor nodes establish peer-to-peer connections with each other via existing networks. Once connected, the receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.

.. toctree::
   :maxdepth: 2

   installation
   getting_started_guide/index
   user_guide/index
   developer_guide
   porting_guide/index
   roadmap/index
   upgrade/index
   contributing
