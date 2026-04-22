---
title: Learning Scheme with EnvDraw
weight: 60
layout: blog
section: Blog
date: "1995-06-15"
summary: "From a high-school nerd running out of math classes to building a self-diagramming metacircular evaluator for UC Berkeley's CS61A. Thirty years later, reborn in WebAssembly."
image: "https://jmacd.github.io/envdraw/envdraw-screenshot.png"
---

I was a high-school nerd who had run out of math classes, so my senior
year I enrolled in a linear algebra class as the nearby university. On
my way from class one day, I ran into a high-school classmate on
campus to use the computer lab, and he showed me how to telnet into a
multiplayer game. Interesting.

I was a first-year undergraduate studying physics. By then I had taken
two computer programming classes, one C++, one Fortran. I could **not
have been less interested** in these courses. A dorm-mate saw me
running FreeBSD and dropped a copy of Structure and Interpretation of
Computer Programs ("SICP", as we say) on my desk, suggested I would
like it. I read as far as the preamble and I knew that he was right.

I was a second-year undergraduate taking the first-year computer
science course, from the SICP textbook with Brian Harvey. Scheme was
**really** my thing. That summer, Brian invited me to work on a
self-diagramming metacircular evaluator, to be used as an instruction
aid for the same course that hooked me.

EnvDraw was published in 1995 and used for the CS61A course at UC
Berkeley in the following years. By metacircular evaluator, we mean
that EnvDraw is a Scheme program that executes Scheme
programs. Students could enter Scheme expressions and see them
evaluated with a live view of the interpreter state, including
box-and-pointer diagrams.

That was not the end of Scheme for me, but almost! (I answered a
`comp.lang.scheme` post the following summer for an internship at Sun
Microsystems, waves at Bryan O'Sullivan.)

Thirty years passed.

Hoot, a Scheme-to-WebAssembly compiler was recently announced. Very
cool! 

[Here](https://github.com/jmacd/envdaraw) is a modern reimplementation
of EnvDraw on Hoot, replacing the old Scheme/Tk display with a D3.js
diagram, compiled into WASM. [Try it in your
browser](https://jmacd.github.io/envdraw).
