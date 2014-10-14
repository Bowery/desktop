// Utils
var lzwDecode = function (minCodeSize, data) {
  // TODO: Now that the GIF parser is a bit different, maybe this should get an array of bytes instead of a String?
  var pos = 0; // Maybe this streaming thing should be merged with the Stream?
  var readCode = function (size) {
    var code = 0;
    for (var i = 0; i < size; i++) {
      if (data.charCodeAt(pos >> 3) & (1 << (pos & 7))) {
        code |= 1 << i;
      }
      pos++;
    }
    return code;
  };

  var output = [];

  var clearCode = 1 << minCodeSize;
  var eoiCode = clearCode + 1;

  var codeSize = minCodeSize + 1;

  var dict = [];

  var clear = function () {
    dict = [];
    codeSize = minCodeSize + 1;
    for (var i = 0; i < clearCode; i++) {
      dict[i] = [i];
    }
    dict[clearCode] = [];
    dict[eoiCode] = null;

  };

  var code;
  var last;

  while (true) {
    last = code;
    code = readCode(codeSize);

    if (code === clearCode) {
      clear();
      continue;
    }
    if (code === eoiCode) break;

    if (code < dict.length) {
      if (last !== clearCode) {
        dict.push(dict[last].concat(dict[code][0]));
      }
    }
    else {
      if (code !== dict.length) throw new Error('Invalid LZW code.');
      dict.push(dict[last].concat(dict[last][0]));
    }
    output.push.apply(output, dict[code]);

    if (dict.length === (1 << codeSize) && codeSize < 12) {
      // If we're at the last code and codeSize is 12, the next code will be a clearCode, and it'll be 12 bits long.
      codeSize++;
    }
  }

  // I don't know if this is technically an error, but some GIFs do it.
  //if (Math.ceil(pos / 8) !== data.length) throw new Error('Extraneous LZW bytes.');
  return output;
};

//
function GifStream (data) {
  this.data = data
  this.len = this.data.length
  this.pos = 0
  this.comment = ""
}

GifStream.prototype.readByte = function () {
  if (this.pos >= this.data.length) {
    throw new Error('Attempted to read past end of stream')
  }
  return this.data.charCodeAt(this.pos++) & 0xFF
}

GifStream.prototype.readBytes = function (n) {
  var bytes = []
  for (var i = 0; i < n; i++)
    bytes.push(this.readByte())

  return bytes
}

GifStream.prototype.read = function (n) {
  var out = ''
  for (var i = 0; i < n; i++)
    out += String.fromCharCode(this.readByte())

  return out
}

GifStream.prototype.readUnsigned = function () { // Little-endian swag
  var a = this.readBytes(2)
  return (a[1] << 8) + a[0]
}

GifStream.prototype.parseID = function (callback) {
  var beenCalled = false
  this._parseID(function (result) {
    if (!beenCalled)
      callback(result)
    beenCalled = true
  })
}

GifStream.prototype._parseID = function (callback) {
  var st = this
  var result = {}


  var bitsToNum = function (ba) {
    return ba.reduce(function (s, n) {
      return s * 2 + n
    }, 0)
  }

  var byteToBitArr = function (bite) {
    var a = []
    for (var i = 7; i >= 0; i--)
      a.push(!!(bite & (1 << i)))

    return a
  }

  var parseCT = function (entries) {
    var ct = []
    for (var i = 0; i < entries; i++)
      ct.push(st.readBytes(3))
    return ct
  }

  var readSubBlocks = function () {
    var size, data = ""

    do {
      size = st.readByte()
      data += st.read(size)
    } while (size !== 0);

    return data
  }

  var parseHeader = function () {
    var hdr = {}
    hdr.sig = st.read(3)
    hdr.ver = st.read(3)
    if (hdr.sig !== 'GIF') throw new Error('Not a GIF file.')

    hdr.width = st.readUnsigned()
    hdr.height = st.readUnsigned()

    var bits = byteToBitArr(st.readByte())
    hdr.gctFlag = bits.shift()
    hdr.colorRes = bitsToNum(bits.splice(0, 3))
    hdr.sorted = bits.shift()
    hdr.gctSize = bitsToNum(bits.splice(0, 3))

    hdr.bgColor = st.readByte()
    hdr.pixelAspectRatio = st.readByte()
    hdr.pixelAspectRatio = st.readByte()
    if (hdr.gctFlag)
      hdr.gct = parseCT(1 << (hdr.gctSize + 1))
    // return hdr ?
  }

  var parseExt = function (block) {
    var parseGCExt = function (block) {
      var blockSize = st.readByte()
      var bits = byteToBitArr(st.readByte())
      block.reserved = bits.splice(0, 3)
      block.disposalMethod = bitsToNum(bits.splice(0, 3));
      block.userInput = bits.shift();
      block.transparencyGiven = bits.shift();

      block.delayTime = st.readUnsigned();

      block.transparencyIndex = st.readByte();

      block.terminator = st.readByte();
      // return block ?
    }


    // The only thing that matters in this whole function
    var parseComExt = function (block) {
      block.comment = readSubBlocks()
      // console.log('parseComExt', block)
      callback && callback(block)
      // return block ?
    }

    var parsePTExt = function (block) {
      // No one *ever* uses this. If you use it, deal with parsing it yourself.
      var blockSize = st.readByte(); // Always 12
      block.ptHeader = st.readBytes(12);
      block.ptData = readSubBlocks();
      // return block ?
    }

    var parseAppExt = function (block) {
      var parseNetscapeExt = function (block) { // ya, netscape. It's 1993.
        var blockSize = st.readByte();
        block.unkown = st.readByte();
        block.iterations = st.readUnsigned()
        block.terminator = st.readByte();
        // return block ?
      }

      var parseUnkownAppExt = function (block) {
        block.appData = readSubBlocks()
        // return block ?
      }

      var blockSize = st.readByte()
      block.identifier = st.read(8)
      block.authCode = st.read(3)
      switch (block.identifier) {
        case 'NETSCAPE':
          parseNetscapeExt(block);
          break;
        default:
          parseUnkownAppExt(block);
          break;
      }
    }

    var parseUnkownExt = function (block) {
      block.data = readSubBlocks()
      // return block?
    }

    block.label = st.readByte()
    switch (block.label) {
      case 0xF9:
        block.extType = 'gce';
        parseGCExt(block);
        break;
      case 0xFE:
        block.extType = 'com';
        parseComExt(block);
        break;
      case 0x01:
        block.extType = 'pte';
        parsePTExt(block);
        break;
      case 0xFF:
        block.extType = 'app';
        parseAppExt(block);
        break;
      default:
        block.extType = 'unknown';
        parseUnknownExt(block);
        break;
    }

  }

  var parseImg = function (img) {
    var deinterlace = function (pixels, width) {
      var newPixels = new Array(pixels.length)
      var rows = pixels.length / width
      var cpRow = function (toRow, fromRow) {
        var fromPixels = pixels.slice(fromRow * width, (fromRow + 1) * width)
        newPixels.splice.apply(newPixels, [toRow * width, width].concat(fromPixels))
      }

      // James Dean Says So
      var offsets = [0, 4, 2, 1]
      var steps = [8, 8, 4, 2]

      var fromRow = 0
      for (var pass = 0; pass < 4; pass++) {
        for (var toRow = offsets[pass]; toRow < rows; toRow += steps[pass]) {
          cpRow(toRow, fromRow)
          fromRow++
        }
      }
      return newPixels
    }

    img.leftPos = st.readUnsigned()
    img.topPos = st.readUnsigned()
    img.width = st.readUnsigned()
    img.height = st.readUnsigned()

    var bits = byteToBitArr(st.readByte())
    img.lctFlag = bits.shift()
    img.interlaced = bits.shift()
    img.sorted = bits.shift()
    img.reserved = bits.splice(0, 2)
    img.lctSize = bitsToNum(bits.splice(0, 3))

    if (img.lctFlag) {
      img.lct = parseCT(1 << (img.lctSize + 1))
    }

    img.lzwMinCodeSize = st.readByte()
    var lzwData = readSubBlocks()

    img.pixels = lzwDecode(img.lzwMinCodeSize, lzwData)

    if (img.interlaced) {
      img.pixels = deinterlace(img.pixels, img.width)
    }
    // return img?
  }

  var parseBlock = function () {
    var block = {}
    block.sentinel = st.readByte()

    switch (String.fromCharCode(block.sentinel)) {
      case '!':
        block.type = 'ext';
        parseExt(block);
        break;
      case ',':
        block.type = 'img';
        parseImg(block);
        break;
      case ';':
        block.type = 'eof';
        break;
      default:
        // console.log('unkown block: 0x' + block.sentinel.toString(16))
        break;
    }

    if (block.type !== 'eof') {
      // return parseBlock()
      setTimeout(parseBlock, 0) // wow, I can't believe I just wrote that
    }
  }

  var parse = function () {
    parseHeader()
    // return parseBlock()
    setTimeout(parseBlock, 0);
  }
  parse();
}
