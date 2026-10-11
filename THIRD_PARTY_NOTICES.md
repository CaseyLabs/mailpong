# Third-Party Notices

MailPong is licensed under the Apache License 2.0 (see `LICENSE` and `NOTICE`). It includes or links the third-party components listed below, each under its own license. This file is an attribution inventory; it does not change any component's license terms. A human should review it before each release (legal review flag L1 in `PLAN.md`).

How this list was produced (ad hoc, 2026-10-11; neither tool is a project dependency):

- Go: `go list -deps ./...` against the module cache, with each module's license file classified by its text. `go run github.com/google/go-licenses@v1.6.0 report ./...` was tried first but fails on the Go 1.26 standard library ("does not have module info"). The Go standard library (BSD-3-Clause, The Go Authors) is linked into the binary as well.
- npm: `npx license-checker-rseidelsohn@4.4.2 --production --csv` in `frontend/`. Development-only tooling (Vite, ESLint, TypeScript and so on) is not shipped and is not listed.
- No listed dependency is copyleft, and none ships an upstream `NOTICE` file, so Apache-2.0 §4(d) requires no propagated notices beyond `NOTICE`. Re-check this when dependencies change.

## Bundled data and assets

### Spellcheck dictionaries (en-US, en-GB, en-CA)

The built app embeds gzipped Hunspell dictionaries from the npm packages `dictionary-en` 4.0.0 (en-US), `dictionary-en-gb` 3.0.0 (en-GB) and `dictionary-en-ca` 3.0.0 (en-CA), distributed through <https://github.com/wooorm/dictionaries>. The packages declare `MIT AND BSD`. The data derives from SCOWL and Hunspell word lists (<http://wordlist.sourceforge.net>); their license texts follow verbatim. The `nspell` engine (MIT) loads them.

#### License of `dictionary-en` (en-US)

```text
en_US Hunspell Dictionary
Version 2020.12.07
Mon Dec 7 20:14:35 2020 -0500 [5ef55f9]
http://wordlist.sourceforge.net

README file for English Hunspell dictionaries derived from SCOWL.

These dictionaries are created using the speller/make-hunspell-dict
script in SCOWL.

The following dictionaries are available:

  en_US (American)
  en_CA (Canadian)
  en_GB-ise (British with "ise" spelling)
  en_GB-ize (British with "ize" spelling)
  en_AU (Australian)

  en_US-large
  en_CA-large
  en_GB-large (with both "ise" and "ize" spelling)
  en_AU-large

The normal (non-large) dictionaries correspond to SCOWL size 60 and,
to encourage consistent spelling, generally only include one spelling
variant for a word.  The large dictionaries correspond to SCOWL size
70 and may include multiple spelling for a word when both variants are
considered almost equal.  The larger dictionaries however (1) have not
been as carefully checked for errors as the normal dictionaries and
thus may contain misspelled or invalid words; and (2) contain
uncommon, yet valid, words that might cause problems as they are
likely to be misspellings of more common words (for example, "ort" and
"calender").

To get an idea of the difference in size, here are 25 random words
only found in the large dictionary for American English:

  Bermejo Freyr's Guenevere Hatshepsut Nottinghamshire arrestment
  crassitudes crural dogwatches errorless fetial flaxseeds godroon
  incretion jalapeño's kelpie kishkes neuroglias pietisms pullulation
  stemwinder stenoses syce thalassic zees

The en_US, en_CA and en_AU are the official dictionaries for Hunspell.
The en_GB and large dictionaries are made available on an experimental
basis.  If you find them useful please send me a quick email at
kevina@gnu.org.

If none of these dictionaries suite you (for example, maybe you want
the normal dictionary that also includes common variants) additional
dictionaries can be generated at http://app.aspell.net/create or by
modifying speller/make-hunspell-dict in SCOWL.  Please do let me know
if you end up publishing a customized dictionary.

If a word is not found in the dictionary or a word is there you think
shouldn't be, you can lookup the word up at http://app.aspell.net/lookup
to help determine why that is.

General comments on these list can be sent directly to me at
kevina@gnu.org or to the wordlist-devel mailing lists
(https://lists.sourceforge.net/lists/listinfo/wordlist-devel).  If you
have specific issues with any of these dictionaries please file a bug
report at https://github.com/kevina/wordlist/issues.

IMPORTANT CHANGES INTRODUCED In 2016.11.20:

New Australian dictionaries thanks to the work of Benjamin Titze
(btitze@protonmail.ch).

IMPORTANT CHANGES INTRODUCED IN 2016.04.24:

The dictionaries are now in UTF-8 format instead of ISO-8859-1.  This
was required to handle smart quotes correctly.

IMPORTANT CHANGES INTRODUCED IN 2016.01.19:

"SET UTF8" was changes to "SET UTF-8" in the affix file as some
versions of Hunspell do not recognize "UTF8".

ADDITIONAL NOTES:

The NOSUGGEST flag was added to certain taboo words.  While I made an
honest attempt to flag the strongest taboo words with the NOSUGGEST
flag, I MAKE NO GUARANTEE THAT I FLAGGED EVERY POSSIBLE TABOO WORD.
The list was originally derived from Németh László, however I removed
some words which, while being considered taboo by some dictionaries,
are not really considered swear words in today's society.

COPYRIGHT, SOURCES, and CREDITS:

The English dictionaries come directly from SCOWL
and is thus under the same copyright of SCOWL.  The affix file is
a heavily modified version of the original english.aff file which was
released as part of Geoff Kuenning's Ispell and as such is covered by
his BSD license.  Part of SCOWL is also based on Ispell thus the
Ispell copyright is included with the SCOWL copyright.

The collective work is Copyright 2000-2018 by Kevin Atkinson as well
as any of the copyrights mentioned below:

  Copyright 2000-2018 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell these word
  lists, the associated scripts, the output created from the scripts,
  and its documentation for any purpose is hereby granted without fee,
  provided that the above copyright notice appears in all copies and
  that both that copyright notice and this permission notice appear in
  supporting documentation. Kevin Atkinson makes no representations
  about the suitability of this array for any purpose. It is provided
  "as is" without express or implied warranty.

Alan Beale <biljir@pobox.com> also deserves special credit as he has,
in addition to providing the 12Dicts package and being a major
contributor to the ENABLE word list, given me an incredible amount of
feedback and created a number of special lists (those found in the
Supplement) in order to help improve the overall quality of SCOWL.

The 10 level includes the 1000 most common English words (according to
the Moby (TM) Words II [MWords] package), a subset of the 1000 most
common words on the Internet (again, according to Moby Words II), and
frequently class 16 from Brian Kelk's "UK English Wordlist
with Frequency Classification".

The MWords package was explicitly placed in the public domain:

    The Moby lexicon project is complete and has
    been place into the public domain. Use, sell,
    rework, excerpt and use in any way on any platform.

    Placing this material on internal or public servers is
    also encouraged. The compiler is not aware of any
    export restrictions so freely distribute world-wide.

    You can verify the public domain status by contacting

    Grady Ward
    3449 Martha Ct.
    Arcata, CA  95521-4884

    grady@netcom.com
    grady@northcoast.com

The "UK English Wordlist With Frequency Classification" is also in the
Public Domain:

  Date: Sat, 08 Jul 2000 20:27:21 +0100
  From: Brian Kelk <Brian.Kelk@cl.cam.ac.uk>

  > I was wondering what the copyright status of your "UK English
  > Wordlist With Frequency Classification" word list as it seems to
  > be lacking any copyright notice.

  There were many many sources in total, but any text marked
  "copyright" was avoided. Locally-written documentation was one
  source. An earlier version of the list resided in a filespace called
  PUBLIC on the University mainframe, because it was considered public
  domain.

  Date: Tue, 11 Jul 2000 19:31:34 +0100

  > So are you saying your word list is also in the public domain?

  That is the intention.

The 20 level includes frequency classes 7-15 from Brian's word list.

The 35 level includes frequency classes 2-6 and words appearing in at
least 11 of 12 dictionaries as indicated in the 12Dicts package.  All
words from the 12Dicts package have had likely inflections added via
my inflection database.

The 12Dicts package and Supplement is in the Public Domain.

The WordNet database, which was used in the creation of the
Inflections database, is under the following copyright:

  This software and database is being provided to you, the LICENSEE,
  by Princeton University under the following license.  By obtaining,
  using and/or copying this software and database, you agree that you
  have read, understood, and will comply with these terms and
  conditions.:

  Permission to use, copy, modify and distribute this software and
  database and its documentation for any purpose and without fee or
  royalty is hereby granted, provided that you agree to comply with
  the following copyright notice and statements, including the
  disclaimer, and that the same appear on ALL copies of the software,
  database and documentation, including modifications that you make
  for internal use or for distribution.

  WordNet 1.6 Copyright 1997 by Princeton University.  All rights
  reserved.

  THIS SOFTWARE AND DATABASE IS PROVIDED "AS IS" AND PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
  IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES OF MERCHANT-
  ABILITY OR FITNESS FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF THE
  LICENSED SOFTWARE, DATABASE OR DOCUMENTATION WILL NOT INFRINGE ANY
  THIRD PARTY PATENTS, COPYRIGHTS, TRADEMARKS OR OTHER RIGHTS.

  The name of Princeton University or Princeton may not be used in
  advertising or publicity pertaining to distribution of the software
  and/or database.  Title to copyright in this software, database and
  any associated documentation shall at all times remain with
  Princeton University and LICENSEE agrees to preserve same.

The 40 level includes words from Alan's 3esl list found in version 4.0
of his 12dicts package.  Like his other stuff the 3esl list is also in the
public domain.

The 50 level includes Brian's frequency class 1, words appearing
in at least 5 of 12 of the dictionaries as indicated in the 12Dicts
package, and uppercase words in at least 4 of the previous 12
dictionaries.  A decent number of proper names is also included: The
top 1000 male, female, and Last names from the 1990 Census report; a
list of names sent to me by Alan Beale; and a few names that I added
myself.  Finally a small list of abbreviations not commonly found in
other word lists is included.

The name files form the Census report is a government document which I
don't think can be copyrighted.

The file special-jargon.50 uses common.lst and word.lst from the
"Unofficial Jargon File Word Lists" which is derived from "The Jargon
File".  All of which is in the Public Domain.  This file also contain
a few extra UNIX terms which are found in the file "unix-terms" in the
special/ directory.

The 55 level includes words from Alan's 2of4brif list found in version
4.0 of his 12dicts package.  Like his other stuff the 2of4brif is also
in the public domain.

The 60 level includes all words appearing in at least 2 of the 12
dictionaries as indicated by the 12Dicts package.

The 70 level includes Brian's frequency class 0 and the 74,550 common
dictionary words from the MWords package.  The common dictionary words,
like those from the 12Dicts package, have had all likely inflections
added.  The 70 level also included the 5desk list from version 4.0 of
the 12Dics package which is in the public domain.

The 80 level includes the ENABLE word list, all the lists in the
ENABLE supplement package (except for ABLE), the "UK Advanced Cryptics
Dictionary" (UKACD), the list of signature words from the YAWL package,
and the 10,196 places list from the MWords package.

The ENABLE package, mainted by M\Cooper <thegrendel@theriver.com>,
is in the Public Domain:

  The ENABLE master word list, WORD.LST, is herewith formally released
  into the Public Domain. Anyone is free to use it or distribute it in
  any manner they see fit. No fee or registration is required for its
  use nor are "contributions" solicited (if you feel you absolutely
  must contribute something for your own peace of mind, the authors of
  the ENABLE list ask that you make a donation on their behalf to your
  favorite charity). This word list is our gift to the Scrabble
  community, as an alternate to "official" word lists. Game designers
  may feel free to incorporate the WORD.LST into their games. Please
  mention the source and credit us as originators of the list. Note
  that if you, as a game designer, use the WORD.LST in your product,
  you may still copyright and protect your product, but you may *not*
  legally copyright or in any way restrict redistribution of the
  WORD.LST portion of your product. This *may* under law restrict your
  rights to restrict your users' rights, but that is only fair.

UKACD, by J Ross Beresford <ross@bryson.demon.co.uk>, is under the
following copyright:

  Copyright (c) J Ross Beresford 1993-1999. All Rights Reserved.

  The following restriction is placed on the use of this publication:
  if The UK Advanced Cryptics Dictionary is used in a software package
  or redistributed in any form, the copyright notice must be
  prominently displayed and the text of this document must be included
  verbatim.

  There are no other restrictions: I would like to see the list
  distributed as widely as possible.

The 95 level includes the 354,984 single words, 256,772 compound
words, 4,946 female names and the 3,897 male names, and 21,986 names
from the MWords package, ABLE.LST from the ENABLE Supplement, and some
additional words found in my part-of-speech database that were not
found anywhere else.

Accent information was taken from UKACD.

The VarCon package was used to create the American, British, Canadian,
and Australian word list.  It is under the following copyright:

  Copyright 2000-2016 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Kevin Atkinson makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Copyright 2016 by Benjamin Titze

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Benjamin Titze makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Since the original words lists come from the Ispell distribution:

  Copyright 1993, Geoff Kuenning, Granada Hills, CA
  All rights reserved.

  Redistribution and use in source and binary forms, with or without
  modification, are permitted provided that the following conditions
  are met:

  1. Redistributions of source code must retain the above copyright
     notice, this list of conditions and the following disclaimer.
  2. Redistributions in binary form must reproduce the above copyright
     notice, this list of conditions and the following disclaimer in the
     documentation and/or other materials provided with the distribution.
  3. All modifications to the source code must be clearly marked as
     such.  Binary redistributions based on modified source code
     must be clearly marked as modified versions in the documentation
     and/or other materials provided with the distribution.
  (clause 4 removed with permission from Geoff Kuenning)
  5. The name of Geoff Kuenning may not be used to endorse or promote
     products derived from this software without specific prior
     written permission.

  THIS SOFTWARE IS PROVIDED BY GEOFF KUENNING AND CONTRIBUTORS ``AS IS'' AND
  ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
  ARE DISCLAIMED.  IN NO EVENT SHALL GEOFF KUENNING OR CONTRIBUTORS BE LIABLE
  FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
  DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
  OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
  HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
  LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
  OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
  SUCH DAMAGE.

Build Date: Mon Dec  7 20:19:27 EST 2020
Wordlist Command: mk-list --accents=strip en_US 60
```

#### License of `dictionary-en-gb` (en-GB)

```text
en_GB-ise Hunspell Dictionary
Version 2020.12.07
Mon Dec 7 20:14:35 2020 -0500 [5ef55f9]
http://wordlist.sourceforge.net

README file for English Hunspell dictionaries derived from SCOWL.

These dictionaries are created using the speller/make-hunspell-dict
script in SCOWL.

The following dictionaries are available:

  en_US (American)
  en_CA (Canadian)
  en_GB-ise (British with "ise" spelling)
  en_GB-ize (British with "ize" spelling)
  en_AU (Australian)

  en_US-large
  en_CA-large
  en_GB-large (with both "ise" and "ize" spelling)
  en_AU-large

The normal (non-large) dictionaries correspond to SCOWL size 60 and,
to encourage consistent spelling, generally only include one spelling
variant for a word.  The large dictionaries correspond to SCOWL size
70 and may include multiple spelling for a word when both variants are
considered almost equal.  The larger dictionaries however (1) have not
been as carefully checked for errors as the normal dictionaries and
thus may contain misspelled or invalid words; and (2) contain
uncommon, yet valid, words that might cause problems as they are
likely to be misspellings of more common words (for example, "ort" and
"calender").

To get an idea of the difference in size, here are 25 random words
only found in the large dictionary for American English:

  Bermejo Freyr's Guenevere Hatshepsut Nottinghamshire arrestment
  crassitudes crural dogwatches errorless fetial flaxseeds godroon
  incretion jalapeño's kelpie kishkes neuroglias pietisms pullulation
  stemwinder stenoses syce thalassic zees

The en_US, en_CA and en_AU are the official dictionaries for Hunspell.
The en_GB and large dictionaries are made available on an experimental
basis.  If you find them useful please send me a quick email at
kevina@gnu.org.

If none of these dictionaries suite you (for example, maybe you want
the normal dictionary that also includes common variants) additional
dictionaries can be generated at http://app.aspell.net/create or by
modifying speller/make-hunspell-dict in SCOWL.  Please do let me know
if you end up publishing a customized dictionary.

If a word is not found in the dictionary or a word is there you think
shouldn't be, you can lookup the word up at http://app.aspell.net/lookup
to help determine why that is.

General comments on these list can be sent directly to me at
kevina@gnu.org or to the wordlist-devel mailing lists
(https://lists.sourceforge.net/lists/listinfo/wordlist-devel).  If you
have specific issues with any of these dictionaries please file a bug
report at https://github.com/kevina/wordlist/issues.

IMPORTANT CHANGES INTRODUCED In 2016.11.20:

New Australian dictionaries thanks to the work of Benjamin Titze
(btitze@protonmail.ch).

IMPORTANT CHANGES INTRODUCED IN 2016.04.24:

The dictionaries are now in UTF-8 format instead of ISO-8859-1.  This
was required to handle smart quotes correctly.

IMPORTANT CHANGES INTRODUCED IN 2016.01.19:

"SET UTF8" was changes to "SET UTF-8" in the affix file as some
versions of Hunspell do not recognize "UTF8".

ADDITIONAL NOTES:

The NOSUGGEST flag was added to certain taboo words.  While I made an
honest attempt to flag the strongest taboo words with the NOSUGGEST
flag, I MAKE NO GUARANTEE THAT I FLAGGED EVERY POSSIBLE TABOO WORD.
The list was originally derived from Németh László, however I removed
some words which, while being considered taboo by some dictionaries,
are not really considered swear words in today's society.

COPYRIGHT, SOURCES, and CREDITS:

The English dictionaries come directly from SCOWL
and is thus under the same copyright of SCOWL.  The affix file is
a heavily modified version of the original english.aff file which was
released as part of Geoff Kuenning's Ispell and as such is covered by
his BSD license.  Part of SCOWL is also based on Ispell thus the
Ispell copyright is included with the SCOWL copyright.

The collective work is Copyright 2000-2018 by Kevin Atkinson as well
as any of the copyrights mentioned below:

  Copyright 2000-2018 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell these word
  lists, the associated scripts, the output created from the scripts,
  and its documentation for any purpose is hereby granted without fee,
  provided that the above copyright notice appears in all copies and
  that both that copyright notice and this permission notice appear in
  supporting documentation. Kevin Atkinson makes no representations
  about the suitability of this array for any purpose. It is provided
  "as is" without express or implied warranty.

Alan Beale <biljir@pobox.com> also deserves special credit as he has,
in addition to providing the 12Dicts package and being a major
contributor to the ENABLE word list, given me an incredible amount of
feedback and created a number of special lists (those found in the
Supplement) in order to help improve the overall quality of SCOWL.

The 10 level includes the 1000 most common English words (according to
the Moby (TM) Words II [MWords] package), a subset of the 1000 most
common words on the Internet (again, according to Moby Words II), and
frequently class 16 from Brian Kelk's "UK English Wordlist
with Frequency Classification".

The MWords package was explicitly placed in the public domain:

    The Moby lexicon project is complete and has
    been place into the public domain. Use, sell,
    rework, excerpt and use in any way on any platform.

    Placing this material on internal or public servers is
    also encouraged. The compiler is not aware of any
    export restrictions so freely distribute world-wide.

    You can verify the public domain status by contacting

    Grady Ward
    3449 Martha Ct.
    Arcata, CA  95521-4884

    grady@netcom.com
    grady@northcoast.com

The "UK English Wordlist With Frequency Classification" is also in the
Public Domain:

  Date: Sat, 08 Jul 2000 20:27:21 +0100
  From: Brian Kelk <Brian.Kelk@cl.cam.ac.uk>

  > I was wondering what the copyright status of your "UK English
  > Wordlist With Frequency Classification" word list as it seems to
  > be lacking any copyright notice.

  There were many many sources in total, but any text marked
  "copyright" was avoided. Locally-written documentation was one
  source. An earlier version of the list resided in a filespace called
  PUBLIC on the University mainframe, because it was considered public
  domain.

  Date: Tue, 11 Jul 2000 19:31:34 +0100

  > So are you saying your word list is also in the public domain?

  That is the intention.

The 20 level includes frequency classes 7-15 from Brian's word list.

The 35 level includes frequency classes 2-6 and words appearing in at
least 11 of 12 dictionaries as indicated in the 12Dicts package.  All
words from the 12Dicts package have had likely inflections added via
my inflection database.

The 12Dicts package and Supplement is in the Public Domain.

The WordNet database, which was used in the creation of the
Inflections database, is under the following copyright:

  This software and database is being provided to you, the LICENSEE,
  by Princeton University under the following license.  By obtaining,
  using and/or copying this software and database, you agree that you
  have read, understood, and will comply with these terms and
  conditions.:

  Permission to use, copy, modify and distribute this software and
  database and its documentation for any purpose and without fee or
  royalty is hereby granted, provided that you agree to comply with
  the following copyright notice and statements, including the
  disclaimer, and that the same appear on ALL copies of the software,
  database and documentation, including modifications that you make
  for internal use or for distribution.

  WordNet 1.6 Copyright 1997 by Princeton University.  All rights
  reserved.

  THIS SOFTWARE AND DATABASE IS PROVIDED "AS IS" AND PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
  IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES OF MERCHANT-
  ABILITY OR FITNESS FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF THE
  LICENSED SOFTWARE, DATABASE OR DOCUMENTATION WILL NOT INFRINGE ANY
  THIRD PARTY PATENTS, COPYRIGHTS, TRADEMARKS OR OTHER RIGHTS.

  The name of Princeton University or Princeton may not be used in
  advertising or publicity pertaining to distribution of the software
  and/or database.  Title to copyright in this software, database and
  any associated documentation shall at all times remain with
  Princeton University and LICENSEE agrees to preserve same.

The 40 level includes words from Alan's 3esl list found in version 4.0
of his 12dicts package.  Like his other stuff the 3esl list is also in the
public domain.

The 50 level includes Brian's frequency class 1, words appearing
in at least 5 of 12 of the dictionaries as indicated in the 12Dicts
package, and uppercase words in at least 4 of the previous 12
dictionaries.  A decent number of proper names is also included: The
top 1000 male, female, and Last names from the 1990 Census report; a
list of names sent to me by Alan Beale; and a few names that I added
myself.  Finally a small list of abbreviations not commonly found in
other word lists is included.

The name files form the Census report is a government document which I
don't think can be copyrighted.

The file special-jargon.50 uses common.lst and word.lst from the
"Unofficial Jargon File Word Lists" which is derived from "The Jargon
File".  All of which is in the Public Domain.  This file also contain
a few extra UNIX terms which are found in the file "unix-terms" in the
special/ directory.

The 55 level includes words from Alan's 2of4brif list found in version
4.0 of his 12dicts package.  Like his other stuff the 2of4brif is also
in the public domain.

The 60 level includes all words appearing in at least 2 of the 12
dictionaries as indicated by the 12Dicts package.

The 70 level includes Brian's frequency class 0 and the 74,550 common
dictionary words from the MWords package.  The common dictionary words,
like those from the 12Dicts package, have had all likely inflections
added.  The 70 level also included the 5desk list from version 4.0 of
the 12Dics package which is in the public domain.

The 80 level includes the ENABLE word list, all the lists in the
ENABLE supplement package (except for ABLE), the "UK Advanced Cryptics
Dictionary" (UKACD), the list of signature words from the YAWL package,
and the 10,196 places list from the MWords package.

The ENABLE package, mainted by M\Cooper <thegrendel@theriver.com>,
is in the Public Domain:

  The ENABLE master word list, WORD.LST, is herewith formally released
  into the Public Domain. Anyone is free to use it or distribute it in
  any manner they see fit. No fee or registration is required for its
  use nor are "contributions" solicited (if you feel you absolutely
  must contribute something for your own peace of mind, the authors of
  the ENABLE list ask that you make a donation on their behalf to your
  favorite charity). This word list is our gift to the Scrabble
  community, as an alternate to "official" word lists. Game designers
  may feel free to incorporate the WORD.LST into their games. Please
  mention the source and credit us as originators of the list. Note
  that if you, as a game designer, use the WORD.LST in your product,
  you may still copyright and protect your product, but you may *not*
  legally copyright or in any way restrict redistribution of the
  WORD.LST portion of your product. This *may* under law restrict your
  rights to restrict your users' rights, but that is only fair.

UKACD, by J Ross Beresford <ross@bryson.demon.co.uk>, is under the
following copyright:

  Copyright (c) J Ross Beresford 1993-1999. All Rights Reserved.

  The following restriction is placed on the use of this publication:
  if The UK Advanced Cryptics Dictionary is used in a software package
  or redistributed in any form, the copyright notice must be
  prominently displayed and the text of this document must be included
  verbatim.

  There are no other restrictions: I would like to see the list
  distributed as widely as possible.

The 95 level includes the 354,984 single words, 256,772 compound
words, 4,946 female names and the 3,897 male names, and 21,986 names
from the MWords package, ABLE.LST from the ENABLE Supplement, and some
additional words found in my part-of-speech database that were not
found anywhere else.

Accent information was taken from UKACD.

The VarCon package was used to create the American, British, Canadian,
and Australian word list.  It is under the following copyright:

  Copyright 2000-2016 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Kevin Atkinson makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Copyright 2016 by Benjamin Titze

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Benjamin Titze makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Since the original words lists come from the Ispell distribution:

  Copyright 1993, Geoff Kuenning, Granada Hills, CA
  All rights reserved.

  Redistribution and use in source and binary forms, with or without
  modification, are permitted provided that the following conditions
  are met:

  1. Redistributions of source code must retain the above copyright
     notice, this list of conditions and the following disclaimer.
  2. Redistributions in binary form must reproduce the above copyright
     notice, this list of conditions and the following disclaimer in the
     documentation and/or other materials provided with the distribution.
  3. All modifications to the source code must be clearly marked as
     such.  Binary redistributions based on modified source code
     must be clearly marked as modified versions in the documentation
     and/or other materials provided with the distribution.
  (clause 4 removed with permission from Geoff Kuenning)
  5. The name of Geoff Kuenning may not be used to endorse or promote
     products derived from this software without specific prior
     written permission.

  THIS SOFTWARE IS PROVIDED BY GEOFF KUENNING AND CONTRIBUTORS ``AS IS'' AND
  ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
  ARE DISCLAIMED.  IN NO EVENT SHALL GEOFF KUENNING OR CONTRIBUTORS BE LIABLE
  FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
  DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
  OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
  HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
  LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
  OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
  SUCH DAMAGE.

Build Date: Mon Dec  7 20:19:30 EST 2020
Wordlist Command: mk-list --accents=strip en_GB-ise 60
```

#### License of `dictionary-en-ca` (en-CA)

```text
en_CA Hunspell Dictionary
Version 2020.12.07
Mon Dec 7 20:14:35 2020 -0500 [5ef55f9]
http://wordlist.sourceforge.net

README file for English Hunspell dictionaries derived from SCOWL.

These dictionaries are created using the speller/make-hunspell-dict
script in SCOWL.

The following dictionaries are available:

  en_US (American)
  en_CA (Canadian)
  en_GB-ise (British with "ise" spelling)
  en_GB-ize (British with "ize" spelling)
  en_AU (Australian)

  en_US-large
  en_CA-large
  en_GB-large (with both "ise" and "ize" spelling)
  en_AU-large

The normal (non-large) dictionaries correspond to SCOWL size 60 and,
to encourage consistent spelling, generally only include one spelling
variant for a word.  The large dictionaries correspond to SCOWL size
70 and may include multiple spelling for a word when both variants are
considered almost equal.  The larger dictionaries however (1) have not
been as carefully checked for errors as the normal dictionaries and
thus may contain misspelled or invalid words; and (2) contain
uncommon, yet valid, words that might cause problems as they are
likely to be misspellings of more common words (for example, "ort" and
"calender").

To get an idea of the difference in size, here are 25 random words
only found in the large dictionary for American English:

  Bermejo Freyr's Guenevere Hatshepsut Nottinghamshire arrestment
  crassitudes crural dogwatches errorless fetial flaxseeds godroon
  incretion jalapeño's kelpie kishkes neuroglias pietisms pullulation
  stemwinder stenoses syce thalassic zees

The en_US, en_CA and en_AU are the official dictionaries for Hunspell.
The en_GB and large dictionaries are made available on an experimental
basis.  If you find them useful please send me a quick email at
kevina@gnu.org.

If none of these dictionaries suite you (for example, maybe you want
the normal dictionary that also includes common variants) additional
dictionaries can be generated at http://app.aspell.net/create or by
modifying speller/make-hunspell-dict in SCOWL.  Please do let me know
if you end up publishing a customized dictionary.

If a word is not found in the dictionary or a word is there you think
shouldn't be, you can lookup the word up at http://app.aspell.net/lookup
to help determine why that is.

General comments on these list can be sent directly to me at
kevina@gnu.org or to the wordlist-devel mailing lists
(https://lists.sourceforge.net/lists/listinfo/wordlist-devel).  If you
have specific issues with any of these dictionaries please file a bug
report at https://github.com/kevina/wordlist/issues.

IMPORTANT CHANGES INTRODUCED In 2016.11.20:

New Australian dictionaries thanks to the work of Benjamin Titze
(btitze@protonmail.ch).

IMPORTANT CHANGES INTRODUCED IN 2016.04.24:

The dictionaries are now in UTF-8 format instead of ISO-8859-1.  This
was required to handle smart quotes correctly.

IMPORTANT CHANGES INTRODUCED IN 2016.01.19:

"SET UTF8" was changes to "SET UTF-8" in the affix file as some
versions of Hunspell do not recognize "UTF8".

ADDITIONAL NOTES:

The NOSUGGEST flag was added to certain taboo words.  While I made an
honest attempt to flag the strongest taboo words with the NOSUGGEST
flag, I MAKE NO GUARANTEE THAT I FLAGGED EVERY POSSIBLE TABOO WORD.
The list was originally derived from Németh László, however I removed
some words which, while being considered taboo by some dictionaries,
are not really considered swear words in today's society.

COPYRIGHT, SOURCES, and CREDITS:

The English dictionaries come directly from SCOWL
and is thus under the same copyright of SCOWL.  The affix file is
a heavily modified version of the original english.aff file which was
released as part of Geoff Kuenning's Ispell and as such is covered by
his BSD license.  Part of SCOWL is also based on Ispell thus the
Ispell copyright is included with the SCOWL copyright.

The collective work is Copyright 2000-2018 by Kevin Atkinson as well
as any of the copyrights mentioned below:

  Copyright 2000-2018 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell these word
  lists, the associated scripts, the output created from the scripts,
  and its documentation for any purpose is hereby granted without fee,
  provided that the above copyright notice appears in all copies and
  that both that copyright notice and this permission notice appear in
  supporting documentation. Kevin Atkinson makes no representations
  about the suitability of this array for any purpose. It is provided
  "as is" without express or implied warranty.

Alan Beale <biljir@pobox.com> also deserves special credit as he has,
in addition to providing the 12Dicts package and being a major
contributor to the ENABLE word list, given me an incredible amount of
feedback and created a number of special lists (those found in the
Supplement) in order to help improve the overall quality of SCOWL.

The 10 level includes the 1000 most common English words (according to
the Moby (TM) Words II [MWords] package), a subset of the 1000 most
common words on the Internet (again, according to Moby Words II), and
frequently class 16 from Brian Kelk's "UK English Wordlist
with Frequency Classification".

The MWords package was explicitly placed in the public domain:

    The Moby lexicon project is complete and has
    been place into the public domain. Use, sell,
    rework, excerpt and use in any way on any platform.

    Placing this material on internal or public servers is
    also encouraged. The compiler is not aware of any
    export restrictions so freely distribute world-wide.

    You can verify the public domain status by contacting

    Grady Ward
    3449 Martha Ct.
    Arcata, CA  95521-4884

    grady@netcom.com
    grady@northcoast.com

The "UK English Wordlist With Frequency Classification" is also in the
Public Domain:

  Date: Sat, 08 Jul 2000 20:27:21 +0100
  From: Brian Kelk <Brian.Kelk@cl.cam.ac.uk>

  > I was wondering what the copyright status of your "UK English
  > Wordlist With Frequency Classification" word list as it seems to
  > be lacking any copyright notice.

  There were many many sources in total, but any text marked
  "copyright" was avoided. Locally-written documentation was one
  source. An earlier version of the list resided in a filespace called
  PUBLIC on the University mainframe, because it was considered public
  domain.

  Date: Tue, 11 Jul 2000 19:31:34 +0100

  > So are you saying your word list is also in the public domain?

  That is the intention.

The 20 level includes frequency classes 7-15 from Brian's word list.

The 35 level includes frequency classes 2-6 and words appearing in at
least 11 of 12 dictionaries as indicated in the 12Dicts package.  All
words from the 12Dicts package have had likely inflections added via
my inflection database.

The 12Dicts package and Supplement is in the Public Domain.

The WordNet database, which was used in the creation of the
Inflections database, is under the following copyright:

  This software and database is being provided to you, the LICENSEE,
  by Princeton University under the following license.  By obtaining,
  using and/or copying this software and database, you agree that you
  have read, understood, and will comply with these terms and
  conditions.:

  Permission to use, copy, modify and distribute this software and
  database and its documentation for any purpose and without fee or
  royalty is hereby granted, provided that you agree to comply with
  the following copyright notice and statements, including the
  disclaimer, and that the same appear on ALL copies of the software,
  database and documentation, including modifications that you make
  for internal use or for distribution.

  WordNet 1.6 Copyright 1997 by Princeton University.  All rights
  reserved.

  THIS SOFTWARE AND DATABASE IS PROVIDED "AS IS" AND PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
  IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, PRINCETON
  UNIVERSITY MAKES NO REPRESENTATIONS OR WARRANTIES OF MERCHANT-
  ABILITY OR FITNESS FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF THE
  LICENSED SOFTWARE, DATABASE OR DOCUMENTATION WILL NOT INFRINGE ANY
  THIRD PARTY PATENTS, COPYRIGHTS, TRADEMARKS OR OTHER RIGHTS.

  The name of Princeton University or Princeton may not be used in
  advertising or publicity pertaining to distribution of the software
  and/or database.  Title to copyright in this software, database and
  any associated documentation shall at all times remain with
  Princeton University and LICENSEE agrees to preserve same.

The 40 level includes words from Alan's 3esl list found in version 4.0
of his 12dicts package.  Like his other stuff the 3esl list is also in the
public domain.

The 50 level includes Brian's frequency class 1, words appearing
in at least 5 of 12 of the dictionaries as indicated in the 12Dicts
package, and uppercase words in at least 4 of the previous 12
dictionaries.  A decent number of proper names is also included: The
top 1000 male, female, and Last names from the 1990 Census report; a
list of names sent to me by Alan Beale; and a few names that I added
myself.  Finally a small list of abbreviations not commonly found in
other word lists is included.

The name files form the Census report is a government document which I
don't think can be copyrighted.

The file special-jargon.50 uses common.lst and word.lst from the
"Unofficial Jargon File Word Lists" which is derived from "The Jargon
File".  All of which is in the Public Domain.  This file also contain
a few extra UNIX terms which are found in the file "unix-terms" in the
special/ directory.

The 55 level includes words from Alan's 2of4brif list found in version
4.0 of his 12dicts package.  Like his other stuff the 2of4brif is also
in the public domain.

The 60 level includes all words appearing in at least 2 of the 12
dictionaries as indicated by the 12Dicts package.

The 70 level includes Brian's frequency class 0 and the 74,550 common
dictionary words from the MWords package.  The common dictionary words,
like those from the 12Dicts package, have had all likely inflections
added.  The 70 level also included the 5desk list from version 4.0 of
the 12Dics package which is in the public domain.

The 80 level includes the ENABLE word list, all the lists in the
ENABLE supplement package (except for ABLE), the "UK Advanced Cryptics
Dictionary" (UKACD), the list of signature words from the YAWL package,
and the 10,196 places list from the MWords package.

The ENABLE package, mainted by M\Cooper <thegrendel@theriver.com>,
is in the Public Domain:

  The ENABLE master word list, WORD.LST, is herewith formally released
  into the Public Domain. Anyone is free to use it or distribute it in
  any manner they see fit. No fee or registration is required for its
  use nor are "contributions" solicited (if you feel you absolutely
  must contribute something for your own peace of mind, the authors of
  the ENABLE list ask that you make a donation on their behalf to your
  favorite charity). This word list is our gift to the Scrabble
  community, as an alternate to "official" word lists. Game designers
  may feel free to incorporate the WORD.LST into their games. Please
  mention the source and credit us as originators of the list. Note
  that if you, as a game designer, use the WORD.LST in your product,
  you may still copyright and protect your product, but you may *not*
  legally copyright or in any way restrict redistribution of the
  WORD.LST portion of your product. This *may* under law restrict your
  rights to restrict your users' rights, but that is only fair.

UKACD, by J Ross Beresford <ross@bryson.demon.co.uk>, is under the
following copyright:

  Copyright (c) J Ross Beresford 1993-1999. All Rights Reserved.

  The following restriction is placed on the use of this publication:
  if The UK Advanced Cryptics Dictionary is used in a software package
  or redistributed in any form, the copyright notice must be
  prominently displayed and the text of this document must be included
  verbatim.

  There are no other restrictions: I would like to see the list
  distributed as widely as possible.

The 95 level includes the 354,984 single words, 256,772 compound
words, 4,946 female names and the 3,897 male names, and 21,986 names
from the MWords package, ABLE.LST from the ENABLE Supplement, and some
additional words found in my part-of-speech database that were not
found anywhere else.

Accent information was taken from UKACD.

The VarCon package was used to create the American, British, Canadian,
and Australian word list.  It is under the following copyright:

  Copyright 2000-2016 by Kevin Atkinson

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Kevin Atkinson makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Copyright 2016 by Benjamin Titze

  Permission to use, copy, modify, distribute and sell this array, the
  associated software, and its documentation for any purpose is hereby
  granted without fee, provided that the above copyright notice appears
  in all copies and that both that copyright notice and this permission
  notice appear in supporting documentation. Benjamin Titze makes no
  representations about the suitability of this array for any
  purpose. It is provided "as is" without express or implied warranty.

  Since the original words lists come from the Ispell distribution:

  Copyright 1993, Geoff Kuenning, Granada Hills, CA
  All rights reserved.

  Redistribution and use in source and binary forms, with or without
  modification, are permitted provided that the following conditions
  are met:

  1. Redistributions of source code must retain the above copyright
     notice, this list of conditions and the following disclaimer.
  2. Redistributions in binary form must reproduce the above copyright
     notice, this list of conditions and the following disclaimer in the
     documentation and/or other materials provided with the distribution.
  3. All modifications to the source code must be clearly marked as
     such.  Binary redistributions based on modified source code
     must be clearly marked as modified versions in the documentation
     and/or other materials provided with the distribution.
  (clause 4 removed with permission from Geoff Kuenning)
  5. The name of Geoff Kuenning may not be used to endorse or promote
     products derived from this software without specific prior
     written permission.

  THIS SOFTWARE IS PROVIDED BY GEOFF KUENNING AND CONTRIBUTORS ``AS IS'' AND
  ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
  ARE DISCLAIMED.  IN NO EVENT SHALL GEOFF KUENNING OR CONTRIBUTORS BE LIABLE
  FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
  DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
  OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
  HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
  LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
  OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
  SUCH DAMAGE.

Build Date: Mon Dec  7 20:19:28 EST 2020
Wordlist Command: mk-list --accents=strip en_CA 60
```

### Nunito font

`frontend/src/assets/fonts/nunito-v16-latin-regular.woff2` is Nunito, Copyright 2016 The Nunito Project Authors (contact@sansoxygen.com), licensed under the SIL Open Font License 1.1. The license (also kept at `frontend/src/assets/fonts/OFL.txt`):

```text
Copyright 2016 The Nunito Project Authors (contact@sansoxygen.com),

This Font Software is licensed under the SIL Open Font License, Version 1.1.
This license is copied below, and is also available with a FAQ at:
http://scripts.sil.org/OFL


-----------------------------------------------------------
SIL OPEN FONT LICENSE Version 1.1 - 26 February 2007
-----------------------------------------------------------

PREAMBLE
The goals of the Open Font License (OFL) are to stimulate worldwide
development of collaborative font projects, to support the font creation
efforts of academic and linguistic communities, and to provide a free and
open framework in which fonts may be shared and improved in partnership
with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves. The
fonts, including any derivative works, can be bundled, embedded, 
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works. The fonts and derivatives,
however, cannot be released under any other type of license. The
requirement for fonts to remain under this license does not apply
to any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such. This may
include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components as
distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting -- in part or in whole -- any of the components of the
Original Version, by changing formats or by porting the Font Software to a
new environment.

"Author" refers to any designer, engineer, programmer, technical
writer or other person who contributed to the Font Software.

PERMISSION & CONDITIONS
Permission is hereby granted, free of charge, to any person obtaining
a copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,
in Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
redistributed and/or sold with any software, provided that each copy
contains the above copyright notice and this license. These can be
included either as stand-alone text files, human-readable headers or
in the appropriate machine-readable metadata fields within text or
binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
Name(s) unless explicit written permission is granted by the corresponding
Copyright Holder. This restriction only applies to the primary font name as
presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
Software shall not be used to promote, endorse or advertise any
Modified Version, except to acknowledge the contribution(s) of the
Copyright Holder(s) and the Author(s) or with their explicit written
permission.

5) The Font Software, modified or unmodified, in part or in whole,
must be distributed entirely under this license, and must not be
distributed under any other license. The requirement for fonts to
remain under this license does not apply to any document created
using the Font Software.

TERMINATION
This license becomes null and void if any of the above conditions are
not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
OTHER DEALINGS IN THE FONT SOFTWARE.
```

## Go dependencies (linked into the binary)

| Module | Version | License |
|---|---|---|
| github.com/ProtonMail/go-crypto | v1.5.2 | BSD-3-Clause |
| github.com/aymerick/douceur | v0.2.0 | MIT |
| github.com/cloudflare/circl | v1.6.5 | BSD-3-Clause |
| github.com/dustin/go-humanize | v1.1.0 | MIT |
| github.com/emersion/go-ical | v0.0.0-20250609112844-439c63cef608 | MIT |
| github.com/emersion/go-imap/v2 | v2.0.0-beta.8 | MIT |
| github.com/emersion/go-message | v0.18.2 | MIT |
| github.com/emersion/go-sasl | v0.0.0-20241020182733-b788ff22d5a6 | MIT |
| github.com/emersion/go-vcard | v0.1.0 | MIT |
| github.com/emersion/go-webdav | v0.7.0 | MIT |
| github.com/godbus/dbus/v5 | v5.2.2 | BSD-2-Clause |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause |
| github.com/gorilla/css | v1.0.1 | BSD-3-Clause |
| github.com/icholy/digest | v1.2.0 | MIT |
| github.com/leaanthony/go-ansi-parser | v1.6.1 | MIT |
| github.com/leaanthony/slicer | v1.6.0 | MIT |
| github.com/leaanthony/u | v1.1.1 | MIT |
| github.com/mattn/go-colorable | v0.1.16 | MIT |
| github.com/mattn/go-isatty | v0.0.24 | MIT |
| github.com/microcosm-cc/bluemonday | v1.0.27 | BSD-3-Clause |
| github.com/pkg/errors | v0.9.1 | BSD-2-Clause |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause |
| github.com/rivo/uniseg | v0.4.7 | MIT |
| github.com/rs/zerolog | v1.35.1 | MIT |
| github.com/teambition/rrule-go | v1.8.2 | MIT |
| github.com/teamwork/tnef | v0.0.0-20200108124832-7deabccfdb32 | MIT |
| github.com/wailsapp/wails/v2 | v2.16.0 | MIT |
| github.com/zalando/go-keyring | v0.2.8 | MIT |
| go.mozilla.org/pkcs7 | v0.10.0 | MIT |
| golang.org/x/crypto | v0.57.0 | BSD-3-Clause |
| golang.org/x/image | v0.46.0 | BSD-3-Clause |
| golang.org/x/net | v0.59.0 | BSD-3-Clause |
| golang.org/x/sys | v0.48.0 | BSD-3-Clause |
| golang.org/x/text | v0.42.0 | BSD-3-Clause |
| modernc.org/libc | v1.77.1 | BSD-3-Clause |
| modernc.org/mathutil | v1.7.1 | BSD-3-Clause |
| modernc.org/memory | v1.12.1 | BSD-3-Clause |
| modernc.org/sqlite | v1.60.1 | BSD-3-Clause |
| software.sslmate.com/src/go-pkcs12 | v0.7.3 | BSD-3-Clause |

## npm runtime dependencies (frontend bundle)

| Package | License |
|---|---|
| @esbuild/linux-x64@0.28.2 | MIT |
| @floating-ui/core@1.8.0 | MIT |
| @floating-ui/dom@1.8.0 | MIT |
| @floating-ui/utils@0.2.12 | MIT |
| @formatjs/ecma402-abstract@2.3.6 | MIT |
| @formatjs/fast-memoize@2.2.7 | MIT |
| @formatjs/icu-messageformat-parser@2.11.4 | MIT |
| @formatjs/icu-skeleton-parser@1.8.16 | MIT |
| @formatjs/intl-localematcher@0.6.2 | MIT |
| @iconify/svelte@5.2.2 | MIT |
| @iconify/types@2.0.0 | MIT |
| @internationalized/date@3.12.4 | Apache-2.0 |
| @jridgewell/gen-mapping@0.3.13 | MIT |
| @jridgewell/remapping@2.3.5 | MIT |
| @jridgewell/resolve-uri@3.1.2 | MIT |
| @jridgewell/sourcemap-codec@1.6.0 | MIT |
| @jridgewell/trace-mapping@0.3.31 | MIT |
| @sveltejs/acorn-typescript@1.0.13 | MIT |
| @swc/helpers@0.5.23 | Apache-2.0 |
| @tanstack/svelte-virtual@3.13.39 | MIT |
| @tanstack/virtual-core@3.17.11 | MIT |
| @tiptap/core@3.31.4 | MIT |
| @tiptap/extension-blockquote@3.31.4 | MIT |
| @tiptap/extension-bold@3.31.4 | MIT |
| @tiptap/extension-bullet-list@3.31.4 | MIT |
| @tiptap/extension-code-block@3.31.4 | MIT |
| @tiptap/extension-code@3.31.4 | MIT |
| @tiptap/extension-document@3.31.4 | MIT |
| @tiptap/extension-dropcursor@3.31.4 | MIT |
| @tiptap/extension-gapcursor@3.31.4 | MIT |
| @tiptap/extension-hard-break@3.31.4 | MIT |
| @tiptap/extension-heading@3.31.4 | MIT |
| @tiptap/extension-horizontal-rule@3.31.4 | MIT |
| @tiptap/extension-image@3.31.4 | MIT |
| @tiptap/extension-italic@3.31.4 | MIT |
| @tiptap/extension-link@3.31.4 | MIT |
| @tiptap/extension-list-item@3.31.4 | MIT |
| @tiptap/extension-list-keymap@3.31.4 | MIT |
| @tiptap/extension-list@3.31.4 | MIT |
| @tiptap/extension-ordered-list@3.31.4 | MIT |
| @tiptap/extension-paragraph@3.31.4 | MIT |
| @tiptap/extension-placeholder@3.31.4 | MIT |
| @tiptap/extension-strike@3.31.4 | MIT |
| @tiptap/extension-table@3.31.4 | MIT |
| @tiptap/extension-text-align@3.31.4 | MIT |
| @tiptap/extension-text-style@3.31.4 | MIT |
| @tiptap/extension-text@3.31.4 | MIT |
| @tiptap/extension-underline@3.31.4 | MIT |
| @tiptap/extensions@3.31.4 | MIT |
| @tiptap/pm@3.31.4 | MIT |
| @tiptap/starter-kit@3.31.4 | MIT |
| @types/estree@1.0.9 | MIT |
| @typescript-eslint/types@8.71.1 | MIT |
| acorn@8.19.0 | MIT |
| aria-query@5.3.1 | Apache-2.0 |
| axobject-query@4.1.0 | Apache-2.0 |
| bits-ui@2.19.5 | MIT |
| cli-color@2.0.4 | ISC |
| clsx@2.1.1 | MIT |
| d@1.0.2 | ISC |
| date-fns-tz@3.2.0 | MIT |
| date-fns@4.4.0 | MIT |
| decimal.js@10.6.0 | MIT |
| deepmerge@4.3.1 | MIT |
| dequal@2.0.3 | MIT |
| devalue@5.9.4 | MIT |
| dictionary-en-ca@3.0.0 | MIT |
| dictionary-en-gb@3.0.0 | MIT |
| dictionary-en@4.0.0 | MIT |
| es5-ext@0.10.64 | ISC |
| es6-iterator@2.0.3 | MIT |
| es6-symbol@3.1.4 | ISC |
| es6-weak-map@2.0.3 | ISC |
| esbuild@0.28.2 | MIT |
| esm-env@1.2.2 | MIT |
| esniff@2.0.1 | ISC |
| esrap@2.4.0 | MIT |
| estree-walker@2.0.2 | MIT |
| event-emitter@0.3.5 | MIT |
| ext@1.7.0 | ISC |
| fflate@0.8.3 | MIT |
| globalyzer@0.1.0 | MIT |
| globrex@0.1.2 | MIT |
| inline-style-parser@0.2.7 | MIT |
| intl-messageformat@10.7.18 | BSD-3-Clause |
| is-buffer@2.0.5 | MIT |
| is-promise@2.2.2 | MIT |
| is-reference@3.0.3 | MIT |
| linkifyjs@4.3.3 | MIT |
| locate-character@3.0.0 | MIT |
| lru-queue@0.1.0 | MIT |
| lz-string@1.5.0 | MIT |
| magic-string@0.30.21 | MIT |
| memoizee@0.4.17 | ISC |
| mri@1.2.0 | MIT |
| next-tick@1.1.0 | ISC |
| nspell@2.1.5 | MIT |
| orderedmap@2.1.1 | MIT |
| prosemirror-changeset@2.4.4 | MIT |
| prosemirror-commands@1.7.2 | MIT |
| prosemirror-dropcursor@1.8.4 | MIT |
| prosemirror-gapcursor@1.4.1 | MIT |
| prosemirror-history@1.5.1 | MIT |
| prosemirror-inputrules@1.5.1 | MIT |
| prosemirror-keymap@1.2.3 | MIT |
| prosemirror-model@1.25.12 | MIT |
| prosemirror-schema-list@1.5.1 | MIT |
| prosemirror-state@1.4.4 | MIT |
| prosemirror-tables@1.8.5 | MIT |
| prosemirror-transform@1.12.2 | MIT |
| prosemirror-view@1.42.6 | MIT |
| rope-sequence@1.3.4 | MIT |
| runed@0.35.1 | MIT |
| sade@1.8.1 | MIT |
| style-to-object@1.0.14 | MIT |
| svelte-i18n@4.0.1 | MIT |
| svelte-toolbelt@0.10.6 | MIT |
| svelte@5.57.2 | MIT |
| tabbable@6.5.0 | MIT |
| tailwind-merge@3.7.0 | MIT |
| tailwind-variants@3.3.1 | MIT |
| tailwindcss@4.3.3 | MIT |
| timers-ext@0.1.8 | ISC |
| tiny-glob@0.2.9 | MIT |
| tslib@2.8.1 | 0BSD |
| type@2.7.3 | ISC |
| w3c-keyname@2.2.8 | MIT |
| zimmerframe@1.1.5 | MIT |
