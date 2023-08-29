<template>
    <div class="input-with-search">
        <el-input id="input-1" v-model="searchKeyword" placeholder="image digest" clearable>
            <template #append>
                <el-button id="search-1" type="primary" @click="handleSearchImages">Search</el-button>
            </template>
        </el-input>
    </div>
    <el-table
            :data="imagesData"
            highlight-current-row
            stripe
            table-layout="fixed"
            style="width: 100%"
            max-height="700"
    >
<!--        可收缩展开内容-->
        <el-table-column fixed type="expand">
            <template #default="props">
                <div>
                  <el-table
                          id="expanded-table"
                          highlight-current-row
                          :data="props.row.layers"
                          :row-class-name="tableRowClassName"
                  >
                      <el-table-column prop="colId" label="Index" align="center" width="80" />
                      <el-table-column prop="instruction" label="Instruction" width="450" />
                      <el-table-column prop="size" label="Size" align="center" width="125" />
                      <el-table-column prop="digest" label="Digest" width="650" />
                      <el-table-column prop="results" label="Results" />
                  </el-table>
                </div>
            </template>
        </el-table-column>
        <el-table-column fixed prop="digest" label="Digest" show-overflow-tooltip width="650" />
        <el-table-column prop="architecture" label="Architecture" align="center" width="120" />
        <el-table-column prop="features" label="Features" align="center" width="100" />
        <el-table-column prop="variant" label="Variant" align="center" width="100" />
        <el-table-column prop="os" label="OS" align="center" width="100" />
        <el-table-column prop="size" label="Size" align="center" width="125" />
        <el-table-column prop="status" label="Status" align="center" width="100" />
        <el-table-column prop="last_pulled" label="Last Pulled" align="center" width="240" />
        <el-table-column prop="last_pushed" label="Last Pushed" align="center" width="240" />
    </el-table>
</template>

<script lang="ts" setup>
import { ref } from 'vue';
import axios from 'axios';

// row-class-name 对每行内容处理的回调函数
const tableRowClassName = ({row, rowIndex}) => {
    row.colId = rowIndex + 1;
};

const page = ref(1);
const pageSize = ref(20);
const searchKeyword = ref('');
const imagesData = ref([]);

function handleSearchImages() {
  console.log("button clicked");
  // reset to page 1 before every search
  page.value = 1;
  getImagesData(searchKeyword.value, page.value, pageSize.value);
}

function getImagesData(search, page, pageSize) {
  // axios get images data responsed from backend API
  axios.get('http://10.10.21.122:23434/images', {
      params: {
          search: search,
          page: page,
          page_size: pageSize
      }
  }).then(response => {
      imagesData.value = response.data['results'];
      // console.log(imagesData.value);
      // console.log(response.data);
  })
  .catch(error => {
      console.log(error);
  });
}

// init web page
getImagesData(searchKeyword.value, page.value, pageSize.value);
</script>

<style scoped>
.input-with-search {
    float: right;
    width: 45%;
}

#search-1 {

}

</style>