#!/usr/bin/env python3
import os

from colorama import init
init()
from colorama import Fore

nodeDirs = os.listdir()

colors = [Fore.WHITE, Fore.GREEN, Fore.CYAN]
nodelines = []
for i,n in enumerate(nodeDirs):
  with open(n + "/stdout") as f:
    lines = f.readlines()
    for l in lines:

      nodelines.append(colors[i] + n + " " + l)

sortednodelines = sorted(nodelines, key=lambda s: s.split(' ')[3])
print("".join(sortednodelines))
