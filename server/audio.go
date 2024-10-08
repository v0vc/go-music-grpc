package main

import (
	"log"
	"os"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

func WriteTags(decTrackPath, coverPath string, isFlac bool, tags map[string]string) error {
	var (
		err     error
		imgData []byte
	)

	if coverPath != "" {
		imgData, err = os.ReadFile(coverPath)
		if err != nil {
			return err
		}
	}

	if isFlac {
		tags["DATE"] = tags["year"]
		tags["PERFORMER"] = tags["albumArtist"]
		tags["TRACKNUMBER"] = tags["track"]
		err = writeFlacTags(decTrackPath, tags, imgData)
	} else {
		err = writeMp3Tags(decTrackPath, tags, imgData)
	}

	return err
}

func writeFlacTags(decTrackPath string, tags map[string]string, imgData []byte) error {
	f, err := flac.ParseFile(decTrackPath)
	if err != nil {
		return err
	}

	tag, idx := extractFLACComment(decTrackPath)
	if tag == nil {
		tag = flacvorbis.New()
	}

	for k, v := range tags {
		er := tag.Add(strings.ToUpper(k), v)
		if er != nil {
			return er
		}
	}

	tagMeta := tag.Marshal()
	if idx > 0 {
		f.Meta[idx] = &tagMeta
	} else {
		f.Meta = append(f.Meta, &tagMeta)
	}

	if imgData != nil {
		picture, er := flacpicture.NewFromImageData(
			flacpicture.PictureTypeFrontCover, "", imgData, "image/jpeg",
		)
		if er != nil {
			log.Println("Tag picture error", er)
		}
		pictureMeta := picture.Marshal()
		f.Meta = append(f.Meta, &pictureMeta)
	}

	return f.Save(decTrackPath)
}

func writeMp3Tags(decTrackPath string, tags map[string]string, imgData []byte) error {
	/*tags["track"] += "/" + tags["trackTotal"]*/
	resolve := map[string]string{
		"album":       "TALB",
		"artist":      "TPE1",
		"albumArtist": "TPE2",
		"genre":       "TCON",
		"title":       "TIT2",
		"track":       "TRCK",
		"year":        "TYER",
	}
	tag, err := id3v2.Open(decTrackPath, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}

	defer func(tag *id3v2.Tag) {
		err = tag.Close()
		if err != nil {
			log.Println(err)
		}
	}(tag)

	for k, v := range tags {
		resolved, ok := resolve[k]
		if ok {
			tag.AddTextFrame(resolved, tag.DefaultEncoding(), v)
		}
	}

	if imgData != nil {
		imgFrame := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3v2.PTFrontCover,
			Picture:     imgData,
		}
		tag.AddAttachedPicture(imgFrame)
	}

	return tag.Save()
}

func extractFLACComment(fileName string) (*flacvorbis.MetaDataBlockVorbisComment, int) {
	file, err := flac.ParseFile(fileName)
	if err != nil {
		log.Println(err)
	}

	var (
		cmt    *flacvorbis.MetaDataBlockVorbisComment
		cmtIdx int
	)

	if file != nil {
		for idx, meta := range file.Meta {
			if meta.Type == flac.VorbisComment {
				cmt, err = flacvorbis.ParseFromMetaDataBlock(*meta)
				cmtIdx = idx
				if err != nil {
					log.Println(err)
				}
			}
		}
	}

	return cmt, cmtIdx
}
