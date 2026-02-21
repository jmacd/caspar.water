Once in a while you find someone still posting on Blogger. They want a
place to post their writing, and they don't care how the site looks
or feels. I've been there.

One of the people still on Blogger is Rob "Commander" Pike, with 30+
posts spanning 20+ years. [One of these posts involves me](https://commandcenter.blogspot.com/2011/08/regular-expressions-in-lexing-and.html).

> Comments extracted from a code review. I've been asked to disseminate them more widely.
>
> I should say something about regular expressions in lexing and parsing.

I should tell the rest of the story.

Google was preparing to release Golang version 1.0 and had begun
promoting a release candidate internally. To be able to submit code in
a language, you had to demonstrate a minimum level of proficiency by
going through a "readability" review. The Go team had opened up a new
readbility review process, all you had to do was write a few hundred
lines of Go and submit a request to the team.

I had been working on Google's C++ logging SDK, and I was interested
in compiling SQL expressions into something that could filter and
aggregate log events before they were recorded. It would start with a
SQL parser, I figured, so I read Effective Go and got started.

From the start, the Go toolchain has included a Yacc parser
generator. You wouldn't write a SQL parser without use of a parser
generator. Surely.

You also wouldn't review a Golang readability request that contained a
Yacc parser generator, except if you are Rob Pike, and that is how I
got Rob as my Golang readability reviewer.

My Goyacc SQL parser definition was proper. I had implemented name
resolution as a pass over the abstract syntax tree for the readability
exercise, and I was having a lot of fun learning Go. I had taken a
shortcut and caused a great offense.

> Regular expressions are hard to write, hard to write well, and can
> be expensive relative to other technologies.

Translation: Regular expressions are easy to read when written well.

> Lexers, on the other hand, are fairly easy to write correctly (if
> not as compactly), and very easy to test.

Translation: A proper lexer definition will take longer to write and
be better in the long run.

> Standard lexing and parsing techniques are so easy to write, so
> general, and so adaptable there's no reason to use regular
> expressions.

Translation: standard lexing and parsing techniques can make it
difficult to find a code reviewer.

