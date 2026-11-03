"""Remove the cover from an EPUB using calibre's own polish API.

Run as: calibre-debug -e strip_cover.py -- /path/to/book.epub

This deliberately does not reimplement EPUB surgery: it drives the same code calibre's
own "Polish" and conversion paths use, so EPUB2/EPUB3 cover detection, SVG-wrapped cover
pages and titlepage conventions are handled by calibre, not by us. See ADR-0003.
"""

import sys

from calibre.ebooks.oeb.polish.container import get_container
from calibre.ebooks.oeb.polish.cover import (
    clean_opf,
    find_cover_image,
    find_cover_image_in_page,
    find_cover_page,
)


def main(path):
    container = get_container(path, tweak_mode=True)

    cover_page = find_cover_page(container)
    cover_image = find_cover_image(container)

    spine_items = tuple(container.spine_items)
    if cover_page is None and spine_items:
        # calibre's own fallback: the first spine item may be a plain image wrapper.
        candidate = container.abspath_to_name(spine_items[0])
        if find_cover_image_in_page(container, candidate) is not None:
            cover_page = candidate

    removed = []
    if cover_page is not None:
        wrapped = find_cover_image_in_page(container, cover_page)
        container.remove_item(cover_page)
        removed.append(cover_page)
        if wrapped is not None:
            container.remove_item(wrapped)
            removed.append(wrapped)

    if cover_image is not None and cover_image not in removed:
        container.remove_item(cover_image)
        removed.append(cover_image)

    # Drops manifest/guide/spine references that now point at nothing.
    list(clean_opf(container))
    container.commit()

    print("booksmith: stripped " + (", ".join(removed) if removed else "nothing (no cover found)"))


if __name__ == "__main__":
    main(sys.argv[-1])
