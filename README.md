##jdpcd

Grabbing PCD(province/city/district) data from [jd](http://www.jd.com/), since the encoding of JSON data from
jd is GBK, so using [conv](https://github.com/mconintet/conv) to convert GBK to UTF-8.

##Usage

1. Exporting logged-in cookies of jd from your browser. You can try using this plugin [cookie.txt export](https://chrome.google.com/webstore/detail/cookietxt-export/lopabhfecdfhgogdbojmaicoicjekelh) if you're using chrome.
Saving the exported cookies into a file, like `cookies.txt` below.

2. Tell "jdpcd" to use the `cookies.txt` file and save output into `out.json`.
```bash
go run jdpcd.go -c cookies.txt -o out.json
```
