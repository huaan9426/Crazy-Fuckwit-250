export type ItemArtworkAsset = Readonly<{
  itemId: string;
  textureKey: string;
  url: string;
}>;

/*
 * `import.meta.glob` 是 Vite 在构建阶段提供的文件索引。它会扫描指定目录，把每张图片转换成
 * 最终可以由浏览器加载的 URL；生产构建时 URL 会自动带上内容哈希，本地开发时则指向源码
 * 文件。这里用文件名作为数据库商品 id，因此新增图片只需要命名为 `<item-id>.jpg`，不需要
 * 再维护第二份手写映射。没有对应文件的商品不会出现在这个 Map 里，Scene 会继续绘制原来的
 * Graphics/Text 卡面，也不会为尚未制作的图片向服务器发送必然失败的 404 请求。
 */
const importedArtworkUrls = import.meta.glob("../assets/items/*.jpg", {
  eager: true,
  import: "default",
  query: "?url"
}) as Record<string, string>;

const ITEM_ARTWORKS: readonly ItemArtworkAsset[] = Object.freeze(
  Object.entries(importedArtworkUrls)
    .map(([path, url]) => {
      const fileName = path.slice(path.lastIndexOf("/") + 1);
      const itemId = fileName.slice(0, -".jpg".length);

      return Object.freeze({
        itemId,
        textureKey: `item-artwork:${itemId}`,
        url
      });
    })
    .sort((first, second) => first.itemId.localeCompare(second.itemId))
);

const ITEM_ARTWORK_BY_ID = new Map(ITEM_ARTWORKS.map((asset) => [asset.itemId, asset]));

export function listItemArtworkAssets(): readonly ItemArtworkAsset[] {
  return ITEM_ARTWORKS;
}

export function findItemArtwork(itemId: string): ItemArtworkAsset | undefined {
  return ITEM_ARTWORK_BY_ID.get(itemId);
}
