# unlimited-cloud-storage
Around 2017 I had the idea that I could theoretically use free image hosters and other services
as my own cloud storage to store pretty much an unlimited amount of data in there.

Its not like I need a huge amount of storage, I just did it as a proof of concept and
because I thought it was a nice project.

Essentially I would take 3-byte blocks from the input file and treat these as the RGB value of a pixel.
So if I take a quadratic image of size x I could store up to x*x*3 bytes in it.

Some image hosters limit the size of images that can be uploaded to them, so if you want to allow for
arbitrary file size you would have to split it up into multiple images and store a list of urls to all those images.
You could also use an image to store the list of urls itself and do it in a similar way how inodes work.

Also instead of encoding data in images you could pretty much use anything where you could store
some information online for free, e.g. chess games.

Back then I wrote the code to encode data as an image in Purebasic as a POC,
but it was slow so I reimplemented it in Python using PIL, this probably increased the performance
by at least an order of magnitude.

Then I wrote a small web application using flask that lets you upload the files to your server,
encodes them on the fly and uploads them to the image hoster and gives you a nice
magnetic link to your server which users can use to access the file.

When trying to access the file, the server would look up what image corresponds to the link,
downloads the image, decodes it back and returns the original file to the user.

But because I still wasn't satisfied with the performance I rewrote it again,
this time from scratch, in go.

I also implemented compression via gzip and encryption via aes256, this not only
increased the performance, but also made sure the image hoster couldn't analyze the data.

So now the upload looks like this:
upload file to server -> compress -> encrypt -> upload to image hoster
And the download looks like this:
download from image hoster -> decrypt -> decompress -> send to user 

